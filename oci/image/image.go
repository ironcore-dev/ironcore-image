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

package image

import (
	"context"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Layer interface {
	Descriptor() ocispec.Descriptor
	Content(ctx context.Context) (io.ReadCloser, error)
}

type Image interface {
	Layer
	Manifest(ctx context.Context) (*ocispec.Manifest, error)
	Config(ctx context.Context) (Layer, error)
	Layers(ctx context.Context) ([]Layer, error)
}

// AsWriteLayers spreads the given Image to all layers.
// The first layer will be the config, then the 'regular' layers and finally the image manifest.
func AsWriteLayers(ctx context.Context, img Image) ([]Layer, error) {
	config, err := img.Config(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config layer: %w", err)
	}

	layers, err := img.Layers(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting image layers: %w", err)
	}

	return append(append([]Layer{config}, layers...), img), nil
}

type Sink interface {
	Push(ctx context.Context, ref string, img Image) error
}

type Source interface {
	Resolve(ctx context.Context, ref string) (Image, error)
}

func Copy(ctx context.Context, dst Sink, src Source, ref string) (Image, error) {
	img, err := src.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("error resolving ref %s: %w", ref, err)
	}

	if err := dst.Push(ctx, ref, img); err != nil {
		return nil, fmt.Errorf("error pushing to ref %s: %w", ref, err)
	}
	return img, nil
}
