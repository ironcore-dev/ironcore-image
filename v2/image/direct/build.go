// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package direct

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/image/pack"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func init() {
	image.RegisterBuilder(Kind, func() image.BuildConfig { return &BuildConfig{} }, Build)
}

type BuildArgs struct {
	Platform ocispec.Platform
	Kernel   xio.Source
	Initrds  []Initrd

	Cmdline   string
	OSRelease string
}

type Initrd struct {
	Opener      xio.Source
	Compression InitrdCompression
}

type InitrdCompression uint8

const (
	InitrdCompressionNone InitrdCompression = iota
	InitrdCompressionGzip
	InitrdCompressionZstd
	InitrdCompressionXz
	InitrdCompressionLz4
	InitrdCompressionUnknown
)

var initrdCompressionToMediaType = map[InitrdCompression]string{
	InitrdCompressionNone: MediaTypeLayerInitrd,
	InitrdCompressionGzip: MediaTypeLayerInitrdGzip,
	InitrdCompressionZstd: MediaTypeLayerInitrdZstd,
	InitrdCompressionXz:   MediaTypeLayerInitrdXz,
	InitrdCompressionLz4:  MediaTypeLayerInitrdLz4,
}

func (ic InitrdCompression) MediaType() (string, error) {
	if mediaType, ok := initrdCompressionToMediaType[ic]; ok {
		return mediaType, nil
	}
	return "", fmt.Errorf("unknown initrd compression type: %v", ic)
}

func BuildWithArgs(ctx context.Context, pusher content.Pusher, args BuildArgs) (ocispec.Descriptor, error) {
	p := pack.NewPacker(pusher).
		ArtifactType(ArtifactType).
		Config(&Config{
			Cmdline:   args.Cmdline,
			OSRelease: args.OSRelease,
			Platform:  args.Platform,
		}, MediaTypeConfig).
		LayerSource(args.Kernel, MediaTypeLayerKernel)

	for i, initrd := range args.Initrds {
		mediaType, err := initrd.Compression.MediaType()
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("[initrd %d]: %w", i, err)
		}

		p.LayerSource(initrd.Opener, mediaType)
	}

	return p.Pack(ctx)
}

func Build(ctx context.Context, pusher content.Pusher, buildCtx image.BuildContext, cfg image.BuildConfig, opts image.BuilderOptions) (ocispec.Descriptor, error) {
	buildCfg, ok := cfg.(*BuildConfig)
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("expected BuildConfig got %T", cfg)
	}

	args, err := BuildArgsFromConfig(buildCtx, buildCfg, opts)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("building build options: %w", err)
	}

	return BuildWithArgs(ctx, pusher, *args)
}

type BuildConfig struct {
	image.BuildConfigTypeMeta `json:",inline"`
	Kernel                    string   `json:"kernel"`
	Initrds                   []string `json:"initrds,omitempty"`

	Cmdline   string `json:"cmdline,omitempty"`
	OSRelease string `json:"osRelease,omitempty"`
}

var extensionToInitrdCompression = map[string]InitrdCompression{
	".initrd": InitrdCompressionNone,
	".cpio":   InitrdCompressionNone,
	".gz":     InitrdCompressionGzip,
	".gzip":   InitrdCompressionGzip,
	".zstd":   InitrdCompressionZstd,
	".xz":     InitrdCompressionXz,
	".lz":     InitrdCompressionLz4,
	".lz4":    InitrdCompressionLz4,
}

func initrdCompressionFromFilename(filename string) InitrdCompression {
	ext := strings.ToLower(filepath.Ext(filename))
	if compression, ok := extensionToInitrdCompression[ext]; ok {
		return compression
	}
	if ext == "" {
		return InitrdCompressionNone
	}
	return InitrdCompressionUnknown
}

func BuildArgsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuilderOptions) (*BuildArgs, error) {
	if opts.Platform == nil {
		return nil, fmt.Errorf("must specify platform")
	}

	kernel := xio.FSFileSource(buildCtx, image.Expand(cfg.Kernel, opts))

	initrds := make([]Initrd, 0, len(cfg.Initrds))
	for _, initrd := range cfg.Initrds {
		initrds = append(initrds, Initrd{
			Compression: initrdCompressionFromFilename(initrd),
			Opener:      xio.FSFileSource(buildCtx, image.Expand(initrd, opts)),
		})
	}

	return &BuildArgs{
		Platform:  *opts.Platform,
		Kernel:    kernel,
		Initrds:   initrds,
		Cmdline:   cfg.Cmdline,
		OSRelease: cfg.OSRelease,
	}, nil
}
