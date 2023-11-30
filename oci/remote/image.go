// Copyright 2021 IronCore authors
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
