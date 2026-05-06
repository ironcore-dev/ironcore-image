// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

type Provider struct {
}

func (Provider) NewConfig() image.Config {
	return &BuildConfig{}
}

func (Provider) Build(ctx context.Context, pusher content.Pusher, buildCtx image.BuildContext, cfg image.Config, opts image.BuildOptions) (ocispec.Descriptor, error) {
	buildCfg, ok := cfg.(*BuildConfig)
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("expected BuildConfig got %T", cfg)
	}

	args, err := BuildOptionsFromConfig(buildCtx, buildCfg, opts)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("building build options: %w", err)
	}

	desc, _, err := Build(ctx, pusher, *args)
	return desc, err
}

func (Provider) Inspect(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (image.Image, error) {
	img, err := Inspect(ctx, fetcher, desc)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func init() {
	image.RegisterProvider(Kind, ArtifactType, Provider{})
}
