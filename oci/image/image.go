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
