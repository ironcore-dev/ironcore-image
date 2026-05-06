// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/qcow2"
	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	"github.com/ironcore-dev/ironcore-image/v2/xos"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

type BuildOptions struct {
	Chain []xio.Source
}

func Build(ctx context.Context, pusher content.Pusher, opts BuildOptions) (ocispec.Descriptor, *Image, error) {
	chain, cleanup, err := rebaseChain(opts.Chain)
	if err != nil {
		return ocispec.Descriptor{}, nil, fmt.Errorf("rebasing chain: %w", err)
	}
	defer func() { _ = cleanup() }()

	img := &Image{}
	desc, err := xcontent.NewBuilder(pusher).
		WithArtifactType(ArtifactType).
		LayersSameMediaType(&img.Chain, chain, LayerQcow2MediaType).
		Config(&img.Config, &Config{}, ConfigMediaType).
		Manifest(&img.Manifest).
		Build(ctx)
	if err != nil {
		return ocispec.Descriptor{}, nil, fmt.Errorf("building image: %w", err)
	}
	return desc, img, nil
}

type BuildConfig struct {
	image.TypeMeta `json:",inline"`
	Chain          []string `json:"chain"`
}

func rebaseChain(in []xio.Source) (chain []xio.Source, cleanup func() error, err error) {
	if len(in) <= 1 {
		return in, func() error { return nil }, nil
	}

	tmpDir, err := os.MkdirTemp("", "disk-build-*")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp directory: %w", err)
	}

	cleanup = func() error { return os.RemoveAll(tmpDir) }

	for i, c := range in {
		f := filepath.Join(tmpDir, fmt.Sprintf("chain-%d.qcow2", i))
		if err := xos.WriteFileOpener(f, c, 0666); err != nil {
			_ = cleanup()
			return nil, nil, fmt.Errorf("[%s] failed to copy to temp file: %w", c, err)
		}

		if err := qcow2.Qcow2().UnsafeRemoveBacking(f); err != nil {
			_ = cleanup()
			return nil, nil, fmt.Errorf("[%s] rebasing: %w", c, err)
		}

		chain = append(chain, xio.FileSource(f))
	}

	return chain, cleanup, nil
}

func BuildOptionsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuildOptions) (*BuildOptions, error) {
	chain := make([]xio.Source, 0, len(cfg.Chain))
	for _, c := range cfg.Chain {
		chain = append(chain, xio.FSFileSource(buildCtx, image.Expand(c, opts)))
	}

	return &BuildOptions{
		Chain: chain,
	}, nil
}
