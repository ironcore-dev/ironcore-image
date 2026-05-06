// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xcontent

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	orascontent "oras.land/oras-go/v2/content"
)

type Inspector struct {
	artifactType string
	manifest     *ocispec.Manifest
	config       *inspectLayer
	layers       []*inspectLayer

	fetcher orascontent.Fetcher

	layersByMediaType map[string]*inspectLayer
}

func NewInspector(fetcher orascontent.Fetcher) *Inspector {
	return &Inspector{
		layersByMediaType: make(map[string]*inspectLayer),
		fetcher:           fetcher,
	}
}

func (i *Inspector) WithArtifactType(artifactType string) *Inspector {
	i.artifactType = artifactType
	return i
}

func (i *Inspector) Manifest(manifest *ocispec.Manifest) *Inspector {
	i.manifest = manifest
	return i
}

type inspectLayer struct {
	optional   bool
	mediaTypes map[string]struct{}
	layerAdder func(desc ocispec.Descriptor) error
}

func newPullerLayer(mediaTypes []string, adder func(ocispec.Descriptor) error, optional bool) *inspectLayer {
	mediaTypeSet := make(map[string]struct{}, len(mediaTypes))
	for _, mt := range mediaTypes {
		mediaTypeSet[mt] = struct{}{}
	}
	return &inspectLayer{
		optional:   optional,
		mediaTypes: mediaTypeSet,
		layerAdder: adder,
	}
}

func (l *inspectLayer) add(desc ocispec.Descriptor) error {
	if _, ok := l.mediaTypes[desc.MediaType]; !ok {
		supported := slices.AppendSeq(make([]string, 0, len(l.mediaTypes)), maps.Keys(l.mediaTypes))
		slices.Sort(supported)
		return fmt.Errorf("unsupported media type %s (supported: %v)", desc.MediaType, supported)
	}
	if err := l.layerAdder(desc); err != nil {
		return fmt.Errorf("adding layer: %w", err)
	}
	return nil
}

func (i *Inspector) Config(p *ocispec.Descriptor, mediaTypes []string) *Inspector {
	i.config = newPullerLayer(mediaTypes, func(desc ocispec.Descriptor) error {
		*p = desc
		return nil
	}, false)
	return i
}

func (i *Inspector) layer(adder func(desc ocispec.Descriptor) error, mediaTypes []string, optional bool) *Inspector {
	layer := newPullerLayer(mediaTypes, adder, optional)
	i.layers = append(i.layers, layer)

	for _, mediaType := range mediaTypes {
		i.layersByMediaType[mediaType] = layer
	}
	return i
}

func (i *Inspector) Layer(dst *ocispec.Descriptor, mediaTypes []string) *Inspector {
	return i.layer(func(desc ocispec.Descriptor) error {
		*dst = desc
		return nil
	}, mediaTypes, false)
}

func (i *Inspector) LayerSlice(dst *[]ocispec.Descriptor, mediaTypes []string) *Inspector {
	return i.layer(func(desc ocispec.Descriptor) error {
		*dst = append(*dst, desc)
		return nil
	}, mediaTypes, false)
}

func (i *Inspector) OptLayerSlice(dst *[]ocispec.Descriptor, mediaTypes []string) *Inspector {
	return i.layer(func(desc ocispec.Descriptor) error {
		*dst = append(*dst, desc)
		return nil
	}, mediaTypes, true)
}

func (i *Inspector) Unmarshal(manifest *ocispec.Manifest) error {
	if manifest.MediaType != ocispec.MediaTypeImageManifest {
		return fmt.Errorf("not an image manifest: %q", manifest.MediaType)
	}

	if i.artifactType != manifest.ArtifactType {
		return fmt.Errorf("unsupported artifact type %q (expected %q)", manifest.ArtifactType, i.artifactType)
	}

	if i.manifest != nil {
		*i.manifest = *manifest
	}

	if i.config != nil {
		if err := i.config.add(manifest.Config); err != nil {
			return err
		}
	}

	missingLayers := make(map[*inspectLayer]struct{})
	for _, layer := range i.layers {
		if layer.optional {
			continue
		}

		missingLayers[layer] = struct{}{}
	}

	for _, desc := range manifest.Layers {
		layer, ok := i.layersByMediaType[desc.MediaType]
		if !ok {
			return fmt.Errorf("unknown layer media type %q", desc.MediaType)
		}

		if err := layer.add(desc); err != nil {
			return fmt.Errorf("layer %s: %w", desc.MediaType, err)
		}

		delete(missingLayers, layer)
	}

	var missingMediaTypes []string
	if len(missingLayers) > 0 {
		for missing := range missingLayers {
			mediaTypes := slices.AppendSeq(make([]string, len(missing.mediaTypes)), maps.Keys(missing.mediaTypes))
			sort.Strings(missingMediaTypes)
			missingMediaTypes = append(missingMediaTypes, strings.Join(mediaTypes, ","))
		}
		return fmt.Errorf("missing layer(s): %v", strings.Join(missingMediaTypes, ","))
	}

	if i.manifest != nil {
		*i.manifest = *manifest
	}

	return nil
}

func (i *Inspector) Inspect(ctx context.Context, desc ocispec.Descriptor) error {
	data, err := orascontent.FetchAll(ctx, i.fetcher, desc)
	if err != nil {
		return fmt.Errorf("fetching content: %w", err)
	}

	manifest := &ocispec.Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return fmt.Errorf("unmarshalling manifest: %w", err)
	}

	return i.Unmarshal(manifest)
}
