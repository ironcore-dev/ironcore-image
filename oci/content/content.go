// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	ociimage "github.com/ironcore-dev/ironcore-image/oci/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

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
	data, err := content.ReadBlob(ctx, i.provider, i.descriptor)
	if err != nil {
		return nil, fmt.Errorf("error reading blob for descriptor %s: %w", i.descriptor.Digest, err)
	}

	manifest := &ocispec.Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("error unmarshaling manifest: %w", err)
	}
	return manifest, nil
}

func (i *image) Config(ctx context.Context) (ociimage.Layer, error) {
	manifest, err := i.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	return Layer(i.provider, manifest.Config), nil
}

func (i *image) Layers(ctx context.Context) ([]ociimage.Layer, error) {
	manifest, err := i.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	layers := make([]ociimage.Layer, 0, len(manifest.Layers))
	for _, desc := range manifest.Layers {
		layers = append(layers, Layer(i.provider, desc))
	}
	return layers, nil
}

func Image(provider content.Provider, desc ocispec.Descriptor) ociimage.Image {
	return &image{layer{provider, desc}}
}
