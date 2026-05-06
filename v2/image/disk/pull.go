// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironcore-dev/ironcore-image/v2/qcow2"
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
		LayerSlice(&img.Chain, []string{LayerQcow2MediaType}).
		Inspect(ctx, desc); err != nil {
		return nil, fmt.Errorf("pulling image: %w", err)
	}
	return img, nil
}

func WriteChain(ctx context.Context, fetcher content.Fetcher, chain []ocispec.Descriptor, filename string) error {
	tmpDir, err := os.MkdirTemp("", "chain-write-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	names := make([]string, 0, len(chain))
	for i, c := range chain {
		name := filepath.Join(tmpDir, fmt.Sprintf("chain-%d.qcow2", i))
		if err := xcontent.FetchWriteFile(ctx, fetcher, c, name, 0666); err != nil {
			return fmt.Errorf("writing chain %d: %w", i, err)
		}

		names = append(names, name)
	}

	return qcow2.Qcow2().WriteChain(names, filename)
}

func ReadConfig(ctx context.Context, s content.Fetcher, desc ocispec.Descriptor) (*Config, error) {
	cfg := &Config{}
	if err := xcontent.FetchJSON(ctx, s, desc, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
