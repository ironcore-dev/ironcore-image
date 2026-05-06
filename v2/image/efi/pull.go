// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package efi

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func Inspect(ctx context.Context, s content.Fetcher, desc ocispec.Descriptor) (*Image, error) {
	img := &Image{}
	if err := xcontent.NewInspector(s).
		WithArtifactType(ArtifactType).
		Manifest(&img.Manifest).
		Config(&img.Config, []string{ConfigMediaType}).
		Layer(&img.Executable, []string{LayerEFIExecutableMediaType}).
		Inspect(ctx, desc); err != nil {
		return nil, fmt.Errorf("inspecting image: %w", err)
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
