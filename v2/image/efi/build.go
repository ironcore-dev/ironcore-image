// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package efi

import (
	"context"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

type BuildOptions struct {
	Executable xio.Source
}

func Build(ctx context.Context, pusher content.Pusher, opts BuildOptions) (ocispec.Descriptor, *Image, error) {
	img := &Image{}
	desc, err := xcontent.NewBuilder(pusher).
		WithArtifactType(ArtifactType).
		Layer(&img.Executable, opts.Executable, LayerEFIExecutableMediaType).
		Config(&img.Config, &Config{}, ConfigMediaType).
		Manifest(&img.Manifest).
		Build(ctx)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	return desc, img, nil
}

type BuildConfig struct {
	image.TypeMeta `json:",inline"`
	Executable     string `json:"executable"`
}

func BuildOptionsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuildOptions) (*BuildOptions, error) {
	executable := xio.FSFileSource(buildCtx, image.Expand(cfg.Executable, opts))
	return &BuildOptions{
		Executable: executable,
	}, nil
}
