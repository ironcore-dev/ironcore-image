package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

var (
	formatsMu     sync.Mutex
	atomicFormats atomic.Value
)

type Image interface {
	Platform() *ocispec.Platform
}

type Config struct {
	Platform *ocispec.Platform
}

var ErrFormat = errors.New("unknown format")

type format struct {
	artifactType string
	decode       func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Image, error)
	decodeConfig func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Config, error)
}

func RegisterFormat(
	artifactType string,
	decode func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Image, error),
	decodeConfig func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Config, error),
) {
	formatsMu.Lock()
	defer formatsMu.Unlock()

	formats, _ := atomicFormats.Load().([]format)
	atomicFormats.Store(append(formats, format{
		artifactType: artifactType,
		decode:       decode,
		decodeConfig: decodeConfig,
	}))
}

func findFormatByArtifactType(artifactType string) (*format, bool) {
	formats, _ := atomicFormats.Load().([]format)
	for _, f := range formats {
		if f.artifactType == artifactType {
			return &f, true
		}
	}
	return nil, false
}

func Decode(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Image, string, error) {
	data, err := content.FetchAll(ctx, fetcher, desc)
	if err != nil {
		return nil, "", err
	}

	manifest := &ocispec.Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, "", err
	}

	f, ok := findFormatByArtifactType(manifest.ArtifactType)
	if !ok {
		return nil, "", fmt.Errorf("%w: unknown artifact type %q", ErrFormat, manifest.ArtifactType)
	}

	img, err := f.decode(ctx, fetcher, desc)
	return img, f.artifactType, err
}

func DecodeConfig(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Config, string, error) {
	data, err := content.FetchAll(ctx, fetcher, desc)
	if err != nil {
		return Config{}, "", err
	}

	manifest := &ocispec.Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return Config{}, "", err
	}

	f, ok := findFormatByArtifactType(manifest.ArtifactType)
	if !ok {
		return Config{}, "", fmt.Errorf("%w: unknown artifact type %q", ErrFormat, manifest.ArtifactType)
	}

	cfg, err := f.decodeConfig(ctx, fetcher, manifest.Config)
	return cfg, f.artifactType, err
}
