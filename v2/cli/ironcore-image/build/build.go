// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ironcore-dev/ironcore-image/v2/cli/ironcore-image/common"
	"github.com/ironcore-dev/ironcore-image/v2/image"
	"github.com/ironcore-dev/ironcore-image/v2/platforms"
	"github.com/ironcore-dev/ironcore-image/v2/ui/btea"
	"github.com/ironcore-dev/ironcore-image/v2/ui/observable"
	"github.com/ironcore-dev/ironcore-image/v2/xoci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	_ "github.com/ironcore-dev/ironcore-image/v2/image/disk"
	_ "github.com/ironcore-dev/ironcore-image/v2/image/efi"
	_ "github.com/ironcore-dev/ironcore-image/v2/image/uki"
)

func DefaultFiles(p string) []string {
	var configs []string
	for _, kind := range image.KnownKinds() {
		config := filepath.Join(p, image.KindDefaultConfigName(kind))
		if _, err := os.Stat(config); err == nil {
			configs = append(configs, config)
		}
	}
	return configs
}

type Options struct {
	Files           []string
	Tags            []string
	DefaultPlatform string
	Platforms       common.Platforms
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringSliceVarP(&o.Files, "file", "f", o.Files, "Configuration files to use.")
	cmd.Flags().StringSliceVarP(&o.Tags, "tag", "t", o.Tags, "Tag the image with the given tag.")
	common.PlatformsVar(cmd.Flags(), &o.Platforms, "platform", o.Platforms, "Platforms to build the image with.")
	cmd.Flags().StringVar(&o.DefaultPlatform, "default-platform", o.DefaultPlatform, "The default platform to use if none is specified for a kind.")
}

func Command(prov common.Provider) *cobra.Command {
	var (
		opts = Options{
			DefaultPlatform: runtime.GOARCH,
		}
	)

	cmd := &cobra.Command{
		Use:   "build PATH",
		Short: fmt.Sprintf("Build an image. Available kinds: %v", image.KnownKinds()),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			buildPath := args[0]

			target, err := prov.Store(ctx)
			if err != nil {
				return err
			}

			return Run(ctx, target, buildPath, opts)
		},
	}

	opts.AddFlags(cmd)

	return cmd
}

func Run(ctx context.Context, store *oci.Store, buildPath string, opts Options) error {
	buildCtx, err := image.OpenOSRootBuildContext(buildPath)
	if err != nil {
		return fmt.Errorf("creating build context: %w", err)
	}
	defer func() { _ = buildCtx.Close() }()

	ui := btea.New(btea.Options{})
	defer ui.Stop()

	pusher := &observable.OCIStore{Store: store}
	ui.MonitorPusher(pusher)

	files := opts.Files
	if len(files) == 0 {
		files = DefaultFiles(buildPath)
	}
	if len(files) == 0 {
		return fmt.Errorf("no build files found in %s", buildPath)
	}

	var (
		descs []ocispec.Descriptor
	)
	for _, cfgFile := range files {
		cfg, err := image.ReadConfigFile(cfgFile, "")
		if err != nil {
			return fmt.Errorf("reading config file %q: %w", cfgFile, err)
		}

		buildPlatforms := opts.Platforms.CombinedFor(cfg.GetKind())
		if len(buildPlatforms) == 0 {
			buildPlatforms = []string{platforms.Format(platforms.DefaultSpec())}
		}

		for _, platform := range buildPlatforms {
			p, err := platforms.Parse(platform)
			if err != nil {
				return fmt.Errorf("parsing platform %q: %w", platform, err)
			}

			desc, err := image.Build(ctx, pusher, buildCtx, cfg, image.BuildOptions{
				OS:           p.OS,
				Architecture: p.Architecture,
				Variant:      p.Variant,
			})
			if err != nil {
				return fmt.Errorf("building image %q: %w", cfgFile, err)
			}

			desc.Platform = &p
			descs = append(descs, desc)
		}
	}

	var desc ocispec.Descriptor
	if len(descs) > 1 {
		indexDesc, err := xoci.PackIndex(ctx, store, descs)
		if err != nil {
			return fmt.Errorf("building index: %w", err)
		}

		desc = indexDesc
	} else {
		desc = descs[0]
	}

	for _, tag := range opts.Tags {
		if !strings.Contains(tag, "/") {
			tag = "localhost/" + tag
		}

		ref, err := registry.ParseReference(tag)
		if err != nil {
			return fmt.Errorf("parsing tag %q: %w", tag, err)
		}

		ref.Reference = ref.ReferenceOrDefault()

		if err := store.Tag(ctx, desc, ref.String()); err != nil {
			return fmt.Errorf("tag %q: %w", ref.String(), err)
		}

		ui.TagEvent(btea.TagEvent{
			Tag: tag,
		})
	}

	return nil
}
