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

package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	ociimage "github.com/onmetal/onmetal-image/oci/image"

	"github.com/opencontainers/go-digest"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func DigestAndSizeFromFile(filename string) (dgst digest.Digest, size int64, err error) {
	fp, err := os.Open(filename)
	if err != nil {
		return "", 0, fmt.Errorf("error opening file %s: %w", filename, err)
	}
	defer func() { _ = fp.Close() }()

	info, err := fp.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("error getting stats of file %s: %w", filename, err)
	}

	dgst, err = digest.FromReader(fp)
	if err != nil {
		return "", 0, fmt.Errorf("error getting digest from file %s: %w", filename, err)
	}

	return dgst, info.Size(), nil
}

func WriteFileToIngester(ctx context.Context, ing content.Ingester, desc ocispec.Descriptor, filename string) (ocispec.Descriptor, error) {
	dgst, size, err := DigestAndSizeFromFile(filename)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	desc.Digest = dgst
	desc.Size = size

	ref := remotes.MakeRefKey(ctx, desc)

	f, err := os.Open(filename)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error opening file %s: %w", filename, err)
	}
	defer func() { _ = f.Close() }()

	if err := content.WriteBlob(ctx, ing, ref, f, desc); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error writing file %s: %w", filename, err)
	}
	return desc, nil
}

func WriteJSONEncodedValueToIngester(ctx context.Context, ing content.Ingester, desc ocispec.Descriptor, v interface{}) (ocispec.Descriptor, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not convert %v to json: %w", v, err)
	}

	return WriteDataToIngester(ctx, ing, desc, data)
}

func WriteDataToIngester(ctx context.Context, ing content.Ingester, desc ocispec.Descriptor, data []byte) (ocispec.Descriptor, error) {
	desc.Size = int64(len(data))
	desc.Digest = digest.FromBytes(data)

	ref := remotes.MakeRefKey(ctx, desc)

	if err := content.WriteBlob(ctx, ing, ref, bytes.NewReader(data), desc); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error writing data: %w", err)
	}
	return desc, nil
}

func ReadJSONBlob(ctx context.Context, provider content.Provider, desc ocispec.Descriptor, v interface{}) error {
	rc, err := provider.ReaderAt(ctx, desc)
	if err != nil {
		return fmt.Errorf("error opening reader: %w", err)
	}
	defer func() { _ = rc.Close() }()

	return json.NewDecoder(io.NewSectionReader(rc, 0, desc.Size)).Decode(v)
}

type closeReader struct {
	io.Reader
	close func() error
}

func (c closeReader) Close() error {
	return c.close()
}

func ReaderAtReadCloser(at content.ReaderAt) io.ReadCloser {
	return closeReader{
		content.NewReader(at),
		at.Close,
	}
}

func WriteLayerToIngester(ctx context.Context, ingester content.Ingester, obj ociimage.Layer) error {
	desc := obj.Descriptor()
	rc, err := obj.Content(ctx)
	if err != nil {
		return fmt.Errorf("error opening content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	ref := remotes.MakeRefKey(ctx, desc)
	if err := content.WriteBlob(ctx, ingester, ref, rc, desc); err != nil {
		return fmt.Errorf("error writing data: %w", err)
	}
	return nil
}

func WriteImageToIngester(ctx context.Context, ingester content.Ingester, img ociimage.Image) error {
	layers, err := ociimage.AsWriteLayers(ctx, img)
	if err != nil {
		return fmt.Errorf("error getting image write layers: %w", err)
	}

	for _, layer := range layers {
		if err := WriteLayerToIngester(ctx, ingester, layer); err != nil {
			return fmt.Errorf("error writing layer %s: %w", layer.Descriptor().Digest, err)
		}
	}

	return nil
}

type layer struct {
	provider   content.Provider
	descriptor ocispec.Descriptor
}

func (o *layer) Descriptor() ocispec.Descriptor {
	return o.descriptor
}

func (o *layer) Content(ctx context.Context) (io.ReadCloser, error) {
	readerAt, err := o.provider.ReaderAt(ctx, o.descriptor)
	if err != nil {
		return nil, err
	}

	return ReaderAtReadCloser(readerAt), nil
}

func Layer(provider content.Provider, descriptor ocispec.Descriptor) ociimage.Layer {
	return &layer{provider, descriptor}
}

type image struct {
	layer
}

func (i *image) Manifest(ctx context.Context) (*ocispec.Manifest, error) {
	manifest := &ocispec.Manifest{}
	if err := ReadJSONBlob(ctx, i.provider, i.descriptor, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func (i *image) Config(ctx context.Context) (ociimage.Layer, error) {
	manifest, err := i.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	return &layer{
		provider:   i.provider,
		descriptor: manifest.Config,
	}, nil
}

func (i *image) Layers(ctx context.Context) ([]ociimage.Layer, error) {
	manifest, err := i.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	layers := make([]ociimage.Layer, 0, len(manifest.Layers))
	for _, desc := range manifest.Layers {
		layers = append(layers, &layer{
			provider:   i.provider,
			descriptor: desc,
		})
	}
	return layers, nil
}

func Image(provider content.Provider, desc ocispec.Descriptor) ociimage.Image {
	return &image{layer{provider, desc}}
}
