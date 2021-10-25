// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imageutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"

	"github.com/opencontainers/image-spec/specs-go"

	"github.com/onmetal/onmetal-image/oci/image"
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
	return &Builder{
		config: BytesLayer(data, opts...),
	}
}

func NewJSONConfigBuilder(v interface{}, opts ...DescriptorOpt) *Builder {
	config, err := JSONValueLayer(v, opts...)
	if err != nil {
		return &Builder{err: err}
	}

	return &Builder{
		config: config,
	}
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
	desc.MediaType = ocispec.MediaTypeImageManifest
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
