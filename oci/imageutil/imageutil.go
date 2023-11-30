// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package imageutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type DescriptorOpt func(desc *ocispec.Descriptor)

func WithMediaType(mediaType string) DescriptorOpt {
	return func(desc *ocispec.Descriptor) {
		desc.MediaType = mediaType
	}
}

func WithAnnotations(annotations map[string]string) DescriptorOpt {
	return func(desc *ocispec.Descriptor) {
		desc.Annotations = annotations
	}
}

func WithPlatform(platform ocispec.Platform) DescriptorOpt {
	return func(desc *ocispec.Descriptor) {
		desc.Platform = &platform
	}
}

func JSONValueLayer(v interface{}, opts ...DescriptorOpt) (image.Layer, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("could not marshal to json: %w", err)
	}

	return BytesLayer(data, opts...), nil
}

type bytesLayer struct {
	desc ocispec.Descriptor
	data []byte
}

func (b *bytesLayer) Descriptor() ocispec.Descriptor {
	return b.desc
}

func (b *bytesLayer) Content(ctx context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(b.data)), nil
}

// BytesLayer creates a new image.Layer from the given data.
// The descriptor digest will be overwritten with the digest obtained from the bytes.
// The descriptor size will be overwritten with the length of the data.
func BytesLayer(data []byte, opts ...DescriptorOpt) image.Layer {
	desc := ocispec.Descriptor{}
	for _, opt := range opts {
		opt(&desc)
	}
	desc.Digest = digest.FromBytes(data)
	desc.Size = int64(len(data))
	return &bytesLayer{
		desc: desc,
		data: data,
	}
}

type fileLayer struct {
	desc ocispec.Descriptor
	path string
}

func (f *fileLayer) Content(ctx context.Context) (io.ReadCloser, error) {
	return os.Open(f.path)
}

func (f *fileLayer) Descriptor() ocispec.Descriptor {
	return f.desc
}

func FileLayer(path string, opts ...DescriptorOpt) (image.Layer, error) {
	desc := ocispec.Descriptor{}
	for _, opt := range opts {
		opt(&desc)
	}

	fp, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer func() { _ = fp.Close() }()

	stat, err := fp.Stat()
	if err != nil {
		return nil, fmt.Errorf("error statting file: %w", err)
	}

	dgst, err := digest.FromReader(fp)
	if err != nil {
		return nil, err
	}

	desc.Size = stat.Size()
	desc.Digest = dgst

	return &fileLayer{
		desc: desc,
		path: path,
	}, nil
}

// ReadLayerContent reads the layer contents into a byte slice.
func ReadLayerContent(ctx context.Context, layer image.Layer) ([]byte, error) {
	rc, err := layer.Content(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	return io.ReadAll(rc)
}
