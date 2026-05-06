// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ironcore-dev/ironcore-image/v2/blob"
	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/image/pack"
	"github.com/ironcore-dev/ironcore-image/v2/qcow2"
	"github.com/ironcore-dev/ironcore-image/v2/xos"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func init() {
	image.RegisterBuilder(Kind, func() image.BuildConfig { return &BuildConfig{} }, Build)
}

type BuildArgs struct {
	Chain    []blob.Digested
	Platform ocispec.Platform
}

func BuildWithArgs(ctx context.Context, pusher content.Pusher, args BuildArgs) (ocispec.Descriptor, error) {
	return pack.NewPacker(pusher).
		ArtifactType(ArtifactType).
		Config(&Config{
			Platform: args.Platform,
		}, MediaTypeConfig).
		LayerSlice(args.Chain, MediaTypeLayerQcow2).
		Pack(ctx)
}

func Build(ctx context.Context, pusher content.Pusher, buildCtx image.BuildContext, cfg image.BuildConfig, opts image.BuilderOptions) (ocispec.Descriptor, error) {
	buildCfg, ok := cfg.(*BuildConfig)
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("expected BuildConfig got %T", cfg)
	}

	args, tmpDir, err := BuildArgsFromConfig(buildCtx, buildCfg, opts)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("building build options: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	return BuildWithArgs(ctx, pusher, *args)
}

type BuildConfig struct {
	image.BuildConfigTypeMeta `json:",inline"`
	Chain                     []string `json:"chain"`
}

func RebaseChain(dir string, fileSys fs.FS, filenames []string) (chain []blob.Digested, err error) {
	if len(filenames) == 0 {
		return nil, nil
	}

	for i, filename := range filenames {
		dstName := filepath.Join(dir, fmt.Sprintf("chain-%d.qcow2", i))
		if err := func() error {
			rc, err := fileSys.Open(filename)
			if err != nil {
				return err
			}
			defer func() { _ = rc.Close() }()

			return xos.WriteFileReader(dstName, rc, 0666)
		}(); err != nil {
			return nil, fmt.Errorf("writing %q: %w", filename, err)
		}

		if err := qcow2.Qcow2().UnsafeRemoveBacking(dstName); err != nil {
			return nil, fmt.Errorf("[%s] rebasing: %w", filename, err)
		}

		b, err := blob.DigestedFromFile(filename)
		if err != nil {
			return nil, fmt.Errorf("[%s] getting blob: %w", filename, err)
		}

		chain = append(chain, b)
	}

	return chain, nil
}

func BuildArgsFromConfig(buildCtx image.BuildContext, cfg *BuildConfig, opts image.BuilderOptions) (args *BuildArgs, tempDir string, retErr error) {
	if opts.Platform == nil {
		return nil, "", fmt.Errorf("must specify platform")
	}

	tmpDir, err := os.MkdirTemp("", "disk-build-*")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp directory: %w", err)
	}
	defer func() {
		if retErr != nil {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	chain, err := RebaseChain(tmpDir, buildCtx, image.ExpandSlice(cfg.Chain, opts))
	if err != nil {
		return nil, "", fmt.Errorf("rebasing chain: %w", err)
	}

	return &BuildArgs{
		Platform: *opts.Platform,
		Chain:    chain,
	}, tmpDir, nil
}
