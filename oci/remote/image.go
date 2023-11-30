// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/containerd/containerd/remotes"
	ociimage "github.com/ironcore-dev/ironcore-image/oci/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type layer struct {
	descriptor ocispec.Descriptor
	fetcher    remotes.Fetcher
}

func (l *layer) Descriptor() ocispec.Descriptor {
	return l.descriptor
}

func (l *layer) Content(ctx context.Context) (io.ReadCloser, error) {
	return l.fetcher.Fetch(ctx, l.descriptor)
}

type image struct {
	layer
	sync.Once
	err      error
	manifest *ocispec.Manifest
}

func (i *image) init(ctx context.Context) {
	i.Once.Do(func() {
		rc, err := i.Content(ctx)
		if err != nil {
			i.err = fmt.Errorf("error getting content: %w", err)
			return
		}
		defer func() { _ = rc.Close() }()

		manifest := &ocispec.Manifest{}
		if err := json.NewDecoder(rc).Decode(manifest); err != nil {
			i.err = fmt.Errorf("could not decode manifest: %w", err)
			return
		}

		i.manifest = manifest
	})
}

func (i *image) Manifest(ctx context.Context) (*ocispec.Manifest, error) {
	i.init(ctx)
	return i.manifest, i.err
}

func (i *image) Config(ctx context.Context) (ociimage.Layer, error) {
	i.init(ctx)
	if i.err != nil {
		return nil, i.err
	}
	return &layer{
		descriptor: i.manifest.Config,
		fetcher:    i.fetcher,
	}, nil
}

func (i *image) Layers(ctx context.Context) ([]ociimage.Layer, error) {
	i.init(ctx)
	if i.err != nil {
		return nil, i.err
	}

	layers := make([]ociimage.Layer, 0, len(i.manifest.Layers))
	for _, desc := range i.manifest.Layers {
		layers = append(layers, &layer{
			descriptor: desc,
			fetcher:    i.fetcher,
		})
	}

	return layers, nil
}

func Image(fetcher remotes.Fetcher, desc ocispec.Descriptor) ociimage.Image {
	return &image{
		layer: layer{
			descriptor: desc,
			fetcher:    fetcher,
		},
		Once: sync.Once{},
	}
}
