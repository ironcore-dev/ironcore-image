// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package imageutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Builder struct {
	err    error
	config image.Layer
	layers []image.Layer
}

func NewBuilder(config image.Layer) *Builder {
	return &Builder{
		config: config,
	}
}

func NewBytesConfigBuilder(data []byte, opts ...DescriptorOpt) *Builder {
	return NewBuilder(BytesLayer(data, opts...))
}

func NewJSONConfigBuilder(v interface{}, opts ...DescriptorOpt) *Builder {
	config, err := JSONValueLayer(v, opts...)
	if err != nil {
		return &Builder{err: err}
	}

	return NewBuilder(config)
}

func (b *Builder) BytesLayer(data []byte, opts ...DescriptorOpt) *Builder {
	if b.err != nil {
		return b
	}

	b.layers = append(b.layers, BytesLayer(data, opts...))
	return b
}

func (b *Builder) FileLayer(path string, opts ...DescriptorOpt) *Builder {
	if b.err != nil {
		return b
	}

	layer, err := FileLayer(path, opts...)
	if err != nil {
		b.err = err
		return b
	}

	b.layers = append(b.layers, layer)
	return b
}

func (b *Builder) Layers(layers ...image.Layer) *Builder {
	if b.err != nil {
		return b
	}

	b.layers = append(b.layers, layers...)
	return b
}

func (b *Builder) Complete(opts ...DescriptorOpt) (image.Image, error) {
	if b.err != nil {
		return nil, b.err
	}

	layers := make([]image.Layer, len(b.layers))
	copy(layers, b.layers)

	layerDescriptors := make([]ocispec.Descriptor, 0, len(b.layers))
	for _, layer := range layers {
		layerDescriptors = append(layerDescriptors, layer.Descriptor())
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: b.config.Descriptor(),
		Layers: layerDescriptors,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("error marshaling image manifest: %w", err)
	}

	desc := ocispec.Descriptor{}
	for _, opt := range opts {
		opt(&desc)
	}
	if desc.MediaType == "" {
		desc.MediaType = ocispec.MediaTypeImageManifest
	}
	desc.Digest = digest.FromBytes(data)
	desc.Size = int64(len(data))

	return &composite{
		descriptor: desc,
		manifest:   manifest,
		config:     b.config,
		layers:     layers,
	}, nil
}

type composite struct {
	descriptor ocispec.Descriptor
	manifest   ocispec.Manifest
	config     image.Layer
	layers     []image.Layer
}

func (c *composite) Descriptor() ocispec.Descriptor {
	return c.descriptor
}

func (c *composite) Content(ctx context.Context) (io.ReadCloser, error) {
	data, err := json.Marshal(c.manifest)
	if err != nil {
		return nil, fmt.Errorf("error marshaling image manifest: %w", err)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (c *composite) Manifest(ctx context.Context) (*ocispec.Manifest, error) {
	return &c.manifest, nil
}

func (c *composite) Config(ctx context.Context) (image.Layer, error) {
	return c.config, nil
}

func (c *composite) Layers(ctx context.Context) ([]image.Layer, error) {
	return c.layers, nil
}

func NewIndexImage(index ocispec.Index) (image.Image, error) {
	data, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("error marshaling index manifest: %w", err)
	}

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}

	emptyConfig := BytesLayer([]byte("{}"), WithMediaType("application/vnd.oci.image.config.v1+json"))
	return &indexImage{
		descriptor: desc,
		index:      index,
		config:     emptyConfig,
		layers:     nil,
	}, nil
}

type indexImage struct {
	descriptor ocispec.Descriptor
	index      ocispec.Index
	config     image.Layer
	layers     []image.Layer
}

func (i *indexImage) Descriptor() ocispec.Descriptor {
	return i.descriptor
}

func (i *indexImage) Content(ctx context.Context) (io.ReadCloser, error) {
	data, err := json.Marshal(i.index)
	if err != nil {
		return nil, fmt.Errorf("error marshaling index manifest: %w", err)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (i *indexImage) Manifest(ctx context.Context) (*ocispec.Manifest, error) {
	return nil, fmt.Errorf("indexImage has no manifest")
}

func (i *indexImage) Index(ctx context.Context) (ocispec.Index, error) {
	return i.index, nil
}

func (i *indexImage) Config(ctx context.Context) (image.Layer, error) {
	return i.config, nil
}

func (i *indexImage) Layers(ctx context.Context) ([]image.Layer, error) {
	return i.layers, nil
}
