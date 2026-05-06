// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xcontent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/ironcore-dev/ironcore-image/v2/xio"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

func PushSource(ctx context.Context, p orascontent.Pusher, mediaType string, src xio.Source) (ocispec.Descriptor, error) {
	desc, err := NewDescriptorFromSource(mediaType, src)
	if err != nil {
		return desc, fmt.Errorf("getting descriptor: %w", err)
	}

	rd, err := src.Open()
	if err != nil {
		return desc, fmt.Errorf("opening: %w", err)
	}
	defer func() { _ = rd.Close() }()

	return desc, p.Push(ctx, desc, rd)
}

func IgnoreAlreadyExists(err error) error {
	if errors.Is(err, errdef.ErrAlreadyExists) {
		return nil
	}
	return err
}

func PushJSON(ctx context.Context, s orascontent.Pusher, v any, mediaType string) (ocispec.Descriptor, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshalling json: %w", err)
	}

	desc := orascontent.NewDescriptorFromBytes(mediaType, data)

	if err := s.Push(ctx, desc, bytes.NewReader(data)); err != nil {
		return desc, fmt.Errorf("pushing descriptor: %w", err)
	}
	return desc, nil
}

type builderLayerGroupValue struct {
	p      *[]ocispec.Descriptor
	layers []BuilderLayer
}

func (b *builderLayerGroupValue) count() int {
	return 1
}

func (b *builderLayerGroupValue) build(ctx context.Context, p orascontent.Pusher, into []ocispec.Descriptor) error {
	type result struct {
		idx  int
		desc ocispec.Descriptor
		err  error
	}
	var (
		wg      sync.WaitGroup
		results = make(chan result)
	)
	for i, layer := range b.layers {
		wg.Go(func() {
			desc, err := pushLayerIfNotExists(ctx, p, &layer)
			results <- result{idx: i, desc: desc, err: err}
		})
	}
	go func() {
		defer close(results)
		wg.Wait()
	}()

	var errs []error
	for res := range results {
		if res.err != nil {
			errs = append(errs, res.err)
			continue
		}

		into[res.idx] = res.desc
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	*b.p = into
	return nil
}

type builderLayerVal interface {
	count() int
	build(ctx context.Context, p orascontent.Pusher, into []ocispec.Descriptor) error
}

type Builder struct {
	pusher       orascontent.Pusher
	artifactType string
	config       *builderLayerValue

	layerValues []builderLayerVal
	layerCount  int

	err error

	manifest *ocispec.Manifest
}

type BuilderLayer struct {
	Source    xio.Source
	MediaType string
}

type builderLayerValue struct {
	p     *ocispec.Descriptor
	layer *BuilderLayer
}

func (b *builderLayerValue) count() int {
	return 1
}

func (b *builderLayerValue) buildSingle(ctx context.Context, p orascontent.Pusher) (ocispec.Descriptor, error) {
	desc, err := pushLayerIfNotExists(ctx, p, b.layer)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	*b.p = desc
	return desc, nil
}

func (b *builderLayerValue) build(ctx context.Context, p orascontent.Pusher, into []ocispec.Descriptor) error {
	desc, err := b.buildSingle(ctx, p)
	if err != nil {
		return err
	}

	into[0] = desc
	return nil
}

func NewBuilder(pusher orascontent.Pusher) *Builder {
	return &Builder{
		pusher: pusher,
	}
}

func (b *Builder) WithArtifactType(artifactType string) *Builder {
	b.artifactType = artifactType
	return b
}

func (b *Builder) layerVal(v builderLayerVal) *Builder {
	b.layerValues = append(b.layerValues, v)
	b.layerCount += v.count()
	return b
}

func (b *Builder) Layer(p *ocispec.Descriptor, opener xio.Source, mediaType string) *Builder {
	return b.layerVal(&builderLayerValue{
		p: p,
		layer: &BuilderLayer{
			Source:    opener,
			MediaType: mediaType,
		},
	})
}

func (b *Builder) Layers(p *[]ocispec.Descriptor, layers []BuilderLayer) *Builder {
	return b.layerVal(&builderLayerGroupValue{
		p:      p,
		layers: layers,
	})
}

func (b *Builder) LayersSameMediaType(p *[]ocispec.Descriptor, openers []xio.Source, mediaType string) *Builder {
	layers := make([]BuilderLayer, 0, len(openers))
	for _, o := range openers {
		layers = append(layers, BuilderLayer{Source: o, MediaType: mediaType})
	}
	return b.Layers(p, layers)
}

func (b *Builder) OptLayer(p *ocispec.Descriptor, opener xio.Source, mediaType string) *Builder {
	if opener != nil {
		b.Layer(p, opener, mediaType)
	}
	return b
}

func (b *Builder) Config(p *ocispec.Descriptor, v any, mediaType string) *Builder {
	data, err := json.Marshal(v)
	if err != nil {
		b.err = errors.Join(b.err, err)
		return b
	}

	b.config = &builderLayerValue{
		p: p,
		layer: &BuilderLayer{
			Source:    xio.BytesSource(data),
			MediaType: mediaType,
		},
	}
	return b
}

func (b *Builder) Manifest(p *ocispec.Manifest) *Builder {
	b.manifest = p
	return b
}

func pushLayerIfNotExists(ctx context.Context, pusher orascontent.Pusher, layer *BuilderLayer) (ocispec.Descriptor, error) {
	desc, err := PushSource(ctx, pusher, layer.MediaType, layer.Source)
	if IgnoreAlreadyExists(err) != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing: %w", err)
	}
	return desc, nil
}

func (b *Builder) pushLayers(ctx context.Context) ([]ocispec.Descriptor, error) {
	var (
		descs   = make([]ocispec.Descriptor, b.layerCount)
		wg      sync.WaitGroup
		offset  int
		results = make(chan error)
	)
	for _, layer := range b.layerValues {
		layerOffset := offset
		wg.Go(func() {
			results <- layer.build(ctx, b.pusher, descs[layerOffset:layerOffset+layer.count()])
		})
		offset += layer.count()
	}
	go func() {
		defer close(results)
		wg.Wait()
	}()

	var errs []error
	for err := range results {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return descs, errors.Join(errs...)
}

// pushBytesIfNotExists pushes data described by desc if it does not exist in the
// target.
func pushBytesIfNotExists(ctx context.Context, pusher orascontent.Pusher, desc ocispec.Descriptor, data []byte) error {
	if ros, ok := pusher.(orascontent.ReadOnlyStorage); ok {
		exists, err := ros.Exists(ctx, desc)
		if err != nil {
			return fmt.Errorf("failed to check existence: %s: %s: %w", desc.Digest.String(), desc.MediaType, err)
		}
		if exists {
			return nil
		}
	}

	if err := pusher.Push(ctx, desc, bytes.NewReader(data)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return fmt.Errorf("failed to push: %s: %s: %w", desc.Digest.String(), desc.MediaType, err)
	}
	return nil
}

func (b *Builder) packManifest(
	ctx context.Context,
	configDescOpt *ocispec.Descriptor,
	layers []ocispec.Descriptor,
) (ocispec.Descriptor, error) {
	var emptyBlobExists bool

	var cfgDesc ocispec.Descriptor
	if configDescOpt != nil {
		cfgDesc = *configDescOpt
	} else {
		// use the empty descriptor for config
		cfgDesc = ocispec.DescriptorEmptyJSON
		configBytes := ocispec.DescriptorEmptyJSON.Data
		// push config
		if err := pushBytesIfNotExists(ctx, b.pusher, cfgDesc, configBytes); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("pushing config: %w", err)
		}
		emptyBlobExists = true
	}

	if len(layers) == 0 {
		// use the empty descriptor as the single layer
		layerDesc := ocispec.DescriptorEmptyJSON
		layerData := ocispec.DescriptorEmptyJSON.Data
		if !emptyBlobExists {
			if err := pushBytesIfNotExists(ctx, b.pusher, layerDesc, layerData); err != nil {
				return ocispec.Descriptor{}, fmt.Errorf("failed to push layer: %w", err)
			}
		}
		layers = []ocispec.Descriptor{layerDesc}
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:    ocispec.MediaTypeImageManifest,
		Layers:       layers,
		ArtifactType: b.artifactType,
		Config:       cfgDesc,
		//Annotations:  annotations,
	}
	*b.manifest = manifest

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshalling manifest: %w", err)
	}

	manifestDesc := orascontent.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, manifestJSON)
	// populate ArtifactType and Annotations of the manifest into manifestDesc
	manifestDesc.ArtifactType = b.artifactType
	//manifestDesc.Annotations = annotations

	if err := pushBytesIfNotExists(ctx, b.pusher, manifestDesc, manifestJSON); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing manifest: %w", err)
	}

	return manifestDesc, nil
}

func (b *Builder) Build(ctx context.Context) (ocispec.Descriptor, error) {
	layers, err := b.pushLayers(ctx)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing layers: %w", err)
	}

	var cfgDesc *ocispec.Descriptor
	if b.config != nil {
		desc, err := b.config.buildSingle(ctx, b.pusher)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("building config: %w", err)
		}

		cfgDesc = &desc
	}

	return b.packManifest(ctx, cfgDesc, layers)
}

func FetchJSON(ctx context.Context, s orascontent.Fetcher, desc ocispec.Descriptor, v any) error {
	data, err := orascontent.FetchAll(ctx, s, desc)
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}
	return nil
}

type fetchSource struct {
	ctx     context.Context
	fetcher orascontent.Fetcher
	desc    ocispec.Descriptor
}

func (s *fetchSource) Size() (int64, error) {
	return s.desc.Size, nil
}

func (s *fetchSource) Open() (io.ReadCloser, error) {
	return s.fetcher.Fetch(s.ctx, s.desc)
}

func FetchSource(ctx context.Context, s orascontent.Fetcher, desc ocispec.Descriptor) xio.Source {
	return &fetchSource{
		ctx:     ctx,
		fetcher: s,
		desc:    desc,
	}
}
