// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

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
		return err
	}
	return nil
}

type Part struct {
	Value Value
}

type Packer struct {
	pusher orascontent.Pusher

	artifactType string
	annotations  map[string]string
	platform     *ocispec.Platform
	config       *Part
	parts        []Part

	err error
}

type Value interface {
	Pack(ctx context.Context, pusher orascontent.Pusher) (ocispec.Descriptor, error)
}

type jsonValue struct {
	v         any
	mediaType string
}

func newJSONValue(v any, mediaType string) *jsonValue {
	return &jsonValue{v, mediaType}
}

func (v *jsonValue) Pack(ctx context.Context, pusher orascontent.Pusher) (ocispec.Descriptor, error) {
	data, err := json.Marshal(v.v)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	desc := orascontent.NewDescriptorFromBytes(v.mediaType, data)
	if err := pushBytesIfNotExists(ctx, pusher, desc, data); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

type descriptorValue ocispec.Descriptor

func newDescriptorValue(desc ocispec.Descriptor) descriptorValue {
	return descriptorValue(desc)
}

func (d descriptorValue) Pack(_ context.Context, _ orascontent.Pusher) (ocispec.Descriptor, error) {
	return ocispec.Descriptor(d), nil
}

type sourceValue struct {
	source    xio.Source
	mediaType string
}

func newSourceValue(source xio.Source, mediaType string) *sourceValue {
	return &sourceValue{
		source:    source,
		mediaType: mediaType,
	}
}

func pushSource(ctx context.Context, pusher orascontent.Pusher, src xio.Source, mediaType string) (ocispec.Descriptor, error) {
	desc, err := xcontent.NewDescriptorFromSource(mediaType, src)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("getting descriptor: %w", err)
	}

	if ros, ok := pusher.(orascontent.ReadOnlyStorage); ok {
		ok, err := ros.Exists(ctx, desc)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("checking if descriptor exists: %w", err)
		}
		if ok {
			return desc, nil
		}
	}

	rc, err := src.Open()
	defer func() { _ = rc.Close() }()

	if err := pusher.Push(ctx, desc, rc); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("pushing: %w", err)
	}
	return desc, nil
}

func (v *sourceValue) Pack(ctx context.Context, pusher orascontent.Pusher) (ocispec.Descriptor, error) {
	return pushSource(ctx, pusher, v.source, v.mediaType)
}

type Config interface {
	GetPlatform() *ocispec.Platform
}

func NewPacker(pusher orascontent.Pusher) *Packer {
	return &Packer{
		pusher: pusher,
	}
}

func (p *Packer) ArtifactType(artifactType string) *Packer {
	p.artifactType = artifactType
	return p
}

func (p *Packer) Platform(platform *ocispec.Platform) *Packer {
	p.platform = platform
	return p
}

func (p *Packer) ConfigVar(value Value) *Packer {
	if p.config != nil {
		panic("config already defined")
	}
	p.config = &Part{Value: value}
	return p
}

func (p *Packer) ConfigDescriptor(desc ocispec.Descriptor) *Packer {
	return p.ConfigVar(newDescriptorValue(desc))
}

func (p *Packer) Config(cfg Config, mediaType string) *Packer {
	return p.Platform(cfg.GetPlatform()).ConfigVar(newJSONValue(cfg, mediaType))
}

func (p *Packer) LayerVar(value Value) *Packer {
	p.parts = append(p.parts, Part{Value: value})
	return p
}

func (p *Packer) Layer(desc ocispec.Descriptor) *Packer {
	return p.LayerVar(newDescriptorValue(desc))
}

func (p *Packer) LayerSource(src xio.Source, mediaType string) *Packer {
	return p.LayerVar(newSourceValue(src, mediaType))
}

func (p *Packer) LayerSourceSlice(srcSlice []xio.Source, mediaType string) *Packer {
	for _, src := range srcSlice {
		p.LayerSource(src, mediaType)
	}
	return p
}

func (p *Packer) pushLayers(ctx context.Context) ([]ocispec.Descriptor, error) {
	var (
		descs     = make([]ocispec.Descriptor, len(p.parts))
		sem       = make(chan struct{}, 3)
		grp, gCtx = errgroup.WithContext(ctx)
	)

	for i, part := range p.parts {
		grp.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			desc, err := part.Value.Pack(gCtx, p.pusher)
			if err == nil {
				descs[i] = desc
			}
			return err
		})
	}
	if err := grp.Wait(); err != nil {
		return nil, err
	}
	return descs, nil
}

func (p *Packer) packManifest(
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
		if err := pushBytesIfNotExists(ctx, p.pusher, cfgDesc, configBytes); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("pushing config: %w", err)
		}
		emptyBlobExists = true
	}

	if len(layers) == 0 {
		// use the empty descriptor as the single layer
		layerDesc := ocispec.DescriptorEmptyJSON
		layerData := ocispec.DescriptorEmptyJSON.Data
		if !emptyBlobExists {
			if err := pushBytesIfNotExists(ctx, p.pusher, layerDesc, layerData); err != nil {
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
		ArtifactType: p.artifactType,
		Config:       cfgDesc,
		Annotations:  p.annotations,
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshalling manifest: %w", err)
	}

	manifestDesc := orascontent.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, manifestJSON)
	// populate ArtifactType, Annotations and Platform of the manifest into manifestDesc
	manifestDesc.ArtifactType = p.artifactType
	manifestDesc.Platform = p.platform
	manifestDesc.Annotations = p.annotations

	if err := pushBytesIfNotExists(ctx, p.pusher, manifestDesc, manifestJSON); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing manifest: %w", err)
	}

	return manifestDesc, nil
}

func (p *Packer) Pack(ctx context.Context) (ocispec.Descriptor, error) {
	layers, err := p.pushLayers(ctx)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing layers: %w", err)
	}

	var cfgDesc *ocispec.Descriptor
	if p.config != nil {
		desc, err := p.config.Value.Pack(ctx, p.pusher)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("packing config: %w", err)
		}

		cfgDesc = &desc
	}

	return p.packManifest(ctx, cfgDesc, layers)
}

type IndexOptions struct {
	ArtifactType string
	Annotations  map[string]string
	Manifests    []ocispec.Descriptor
}

func Index(ctx context.Context, pusher orascontent.Pusher, opts IndexOptions) (ocispec.Descriptor, error) {
	if opts.Manifests == nil {
		// Property is required but can have zero length.
		opts.Manifests = []ocispec.Descriptor{}
	}

	index := &ocispec.Index{
		Versioned:    specs.Versioned{SchemaVersion: 2},
		MediaType:    ocispec.MediaTypeImageIndex,
		ArtifactType: opts.ArtifactType,
		Manifests:    opts.Manifests,
		Annotations:  opts.Annotations,
	}
	data, err := json.Marshal(index)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshalling index: %w", err)
	}

	desc := orascontent.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, data)
	if err := pushBytesIfNotExists(ctx, pusher, desc, data); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing index: %w", err)
	}
	return desc, nil
}
