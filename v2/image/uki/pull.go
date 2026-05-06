// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package uki

import (
	"context"
	"fmt"
	"io"

	"github.com/ironcore-dev/ironcore-image/v2/ukify"
	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func Inspect(ctx context.Context, s content.Fetcher, desc ocispec.Descriptor) (*Image, error) {
	img := &Image{}
	if err := xcontent.NewInspector(s).
		WithArtifactType(ArtifactType).
		Manifest(&img.Manifest).
		Config(&img.Config, []string{MediaTypeConfig}).
		Layer(&img.Kernel, []string{MediaTypeLayerKernel}).
		OptLayerSlice(&img.Initrds, InitrdLayerMediaTypes).
		Layer(&img.Stub, []string{MediaTypeLayerStub}).
		Inspect(ctx, desc); err != nil {
		return nil, fmt.Errorf("pulling image: %w", err)
	}
	return img, nil
}

func ReadConfig(ctx context.Context, s content.Fetcher, desc ocispec.Descriptor) (*Config, error) {
	cfg := &Config{}
	if err := xcontent.FetchJSON(ctx, s, desc, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func BuildUKI(ctx context.Context, s content.Fetcher, img *Image, w io.Writer) error {
	cfg, err := ReadConfig(ctx, s, img.Config)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	initrds := make([]xio.Source, 0, len(img.Initrds))
	for _, initrd := range img.Initrds {
		initrds = append(initrds, xcontent.FetchSource(ctx, s, initrd))
	}

	return ukify.Build(w, ukify.BuildOptions{
		Stub:      xcontent.FetchSource(ctx, s, img.Stub),
		Kernel:    xcontent.FetchSource(ctx, s, img.Kernel),
		Initrds:   initrds,
		Cmdline:   cfg.Cmdline,
		OSRelease: cfg.OSRelease,
	})
}
