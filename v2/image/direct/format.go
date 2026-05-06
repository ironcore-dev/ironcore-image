package direct

import (
	"context"
	"encoding/json"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/image/unpack"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func init() {
	image.RegisterFormat(ArtifactType, Decode, DecodeConfig)
}

type Config struct {
	Cmdline   string `json:"cmdline,omitempty"`
	OSRelease string `json:"osRelease,omitempty"`

	ocispec.Platform
}

func (c *Config) GetPlatform() *ocispec.Platform {
	return &c.Platform
}

type Image struct {
	Config  Config
	Kernel  ocispec.Descriptor
	Initrds []ocispec.Descriptor
}

func (i *Image) Platform() *ocispec.Platform {
	return &i.Config.Platform
}

func DecodeConfig(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (image.Config, error) {
	data, err := content.FetchAll(ctx, fetcher, desc)
	if err != nil {
		return image.Config{}, err
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return image.Config{}, err
	}

	return image.Config{
		Platform: &cfg.Platform,
	}, nil
}

func Decode(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (image.Image, error) {
	img := &Image{}
	err := unpack.NewUnpacker(fetcher).
		ArtifactType(ArtifactType).
		ConfigJSON(&img.Config, []string{MediaTypeConfig}).
		Layer(&img.Kernel, []string{MediaTypeLayerKernel}).
		LayerSlice(&img.Initrds, InitrdLayerMediaTypes).
		Unpack(ctx, desc)
	if err != nil {
		return nil, err
	}

	return img, nil
}
