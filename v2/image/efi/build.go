// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package efi

import (
	"context"
	"fmt"

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
	Executable xio.Source
	Platform   ocispec.Platform
}

func BuildWithArgs(ctx context.Context, pusher content.Pusher, opts BuildArgs) (ocispec.Descriptor, error) {
	return pack.NewPacker(pusher).
		ArtifactType(ArtifactType).
		Config(&Config{
			Platform: opts.Platform,
		}, MediaTypeConfig).
		LayerSource(opts.Executable, MediaTypeLayerEFIExecutable).
		Config(&Config{}, MediaTypeConfig).
		Pack(ctx)
}

type BuildConfig struct {
	image.BuildConfigTypeMeta `json:",inline"`
	Executable                string `json:"executable"`
}

func BuildArgsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuilderOptions) (*BuildArgs, error) {
	if opts.Platform == nil {
		return nil, fmt.Errorf("must specify platform")
	}

	executable := xio.FSFileSource(buildCtx, image.Expand(cfg.Executable, opts))
	return &BuildArgs{
		Executable: executable,
		Platform:   *opts.Platform,
	}, nil
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
