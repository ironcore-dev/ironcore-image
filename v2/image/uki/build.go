// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package uki

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

type BuildArgs struct {
	Kernel  xio.Source
	Initrds []Initrd
	Stub    xio.Source

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

func Build(ctx context.Context, pusher content.Pusher, args BuildArgs) (ocispec.Descriptor, *Image, error) {
	initrds := make([]xcontent.BuilderLayer, 0, len(args.Initrds))
	for i, initrd := range args.Initrds {
		mediaType, err := initrd.Compression.MediaType()
		if err != nil {
			return ocispec.Descriptor{}, nil, fmt.Errorf("[initrd %d]: %w", i, err)
		}

		initrds = append(initrds, xcontent.BuilderLayer{
			Source:    initrd.Opener,
			MediaType: mediaType,
		})
	}

	img := &Image{}
	desc, err := xcontent.NewBuilder(pusher).
		WithArtifactType(ArtifactType).
		Layer(&img.Kernel, args.Kernel, MediaTypeLayerKernel).
		Layers(&img.Initrds, initrds).
		Layer(&img.Stub, args.Stub, MediaTypeLayerStub).
		Config(&img.Config, &Config{
			Cmdline:   args.Cmdline,
			OSRelease: args.OSRelease,
		}, MediaTypeConfig).
		Manifest(&img.Manifest).
		Build(ctx)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	return desc, img, nil
}

type BuildConfig struct {
	image.TypeMeta `json:",inline"`
	Kernel         string   `json:"kernel"`
	Initrds        []string `json:"initrds,omitempty"`
	Stub           string   `json:"stub,omitempty"`

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

func BuildOptionsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuildOptions) (*BuildArgs, error) {
	kernel := xio.FSFileSource(buildCtx, image.Expand(cfg.Kernel, opts))

	initrds := make([]Initrd, 0, len(cfg.Initrds))
	for _, initrd := range cfg.Initrds {
		initrds = append(initrds, Initrd{
			Compression: initrdCompressionFromFilename(initrd),
			Opener:      xio.FSFileSource(buildCtx, image.Expand(initrd, opts)),
		})
	}

	var stub xio.Source
	if cfg.Stub != "" {
		stub = xio.FSFileSource(buildCtx, image.Expand(cfg.Stub, opts))
	}

	return &BuildArgs{
		Kernel:    kernel,
		Initrds:   initrds,
		Stub:      stub,
		Cmdline:   cfg.Cmdline,
		OSRelease: cfg.OSRelease,
	}, nil
}
