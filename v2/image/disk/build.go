// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/image/pack"
	"github.com/ironcore-dev/ironcore-image/v2/qcow2"
	"github.com/ironcore-dev/ironcore-image/v2/xio"
	"github.com/ironcore-dev/ironcore-image/v2/xos"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func init() {
	image.RegisterBuilder(Kind, func() image.BuildConfig { return &BuildConfig{} }, Build)
}

type BuildArgs struct {
	Chain    []xio.Source
	Platform ocispec.Platform
}

func BuildWithArgs(ctx context.Context, pusher content.Pusher, args BuildArgs) (ocispec.Descriptor, error) {
	chain, cleanup, err := rebaseChain(args.Chain)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("rebasing chain: %w", err)
	}
	defer func() { _ = cleanup() }()

	return pack.NewPacker(pusher).
		ArtifactType(ArtifactType).
		Config(&Config{
			Platform: args.Platform,
		}, MediaTypeConfig).
		LayerSourceSlice(chain, MediaTypeLayerQcow2).
		Pack(ctx)
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
	Chain                     []string `json:"chain"`
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

func BuildArgsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuilderOptions) (*BuildArgs, error) {
	if opts.Platform == nil {
		return nil, fmt.Errorf("must specify platform")
	}

	chain := make([]xio.Source, 0, len(cfg.Chain))
	for _, c := range cfg.Chain {
		chain = append(chain, xio.FSFileSource(buildCtx, image.Expand(c, opts)))
	}

	return &BuildArgs{
		Platform: *opts.Platform,
		Chain:    chain,
	}, nil
}
