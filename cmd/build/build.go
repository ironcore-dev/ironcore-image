// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencontainers/image-spec/specs-go"

	ironcoreimage "github.com/ironcore-dev/ironcore-image"
	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/ironcore-dev/ironcore-image/oci/imageutil"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

type ArchConfig struct {
	Arch      *string
	RootFS    *string
	InitRAMFS *string
	Kernel    *string
	SquashFS  *string
	UKI       *string
	ISO       *string
	CMDLine   *string
}

type archConfigs []ArchConfig

func (ac *archConfigs) String() string {
	return fmt.Sprintf("%v", *ac)
}

func (ac *archConfigs) Set(value string) error {
	parts := strings.Split(value, ",")
	config := ArchConfig{}

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid format in --arch-config: %s", part)
		}
		key, val := kv[0], kv[1]
		switch key {
		case "arch":
			config.Arch = &val
		case "rootfs":
			config.RootFS = &val
		case "initramfs":
			config.InitRAMFS = &val
		case "kernel":
			config.Kernel = &val
		case "squashfs":
			config.SquashFS = &val
		case "uki":
			config.UKI = &val
		case "iso":
			config.ISO = &val
		case "cmdline":
			config.CMDLine = &val
		default:
			return fmt.Errorf("unknown field %q in --config", key)
		}
	}
	*ac = append(*ac, config)
	return nil
}

func (ac *archConfigs) Type() string {
	return "archConfig"
}

func Command(storeFactory common.StoreFactory) *cobra.Command {
	var (
		tagName string

		archConfigs archConfigs
		annotations map[string]string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image and store it to the local store with an optional tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, storeFactory, tagName, archConfigs, annotations)
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Optional tag of image.")
	cmd.Flags().Var(&archConfigs, "config", "Architecture-specific configuration in the format 'arch=amd64,rootfs=path,initramfs=path'. Can be specified multiple times.")
	cmd.Flags().StringToStringVar(&annotations, "annotations", nil, "Annotations for the IndexManifest in the format 'key=value'. Can specify multiple key-value pairs.")

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	tagName string,
	archConfigs archConfigs,
	annotations map[string]string,
) error {
	s, err := storeFactory()
	manifests := make([]ocispec.Descriptor, 0, len(archConfigs))
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	for _, config := range archConfigs {
		img, err := buildImage(ctx, config.RootFS, config.SquashFS, config.InitRAMFS, config.Kernel, config.UKI, config.ISO, config.CMDLine)
		if err != nil {
			return fmt.Errorf("error building image for arch %s: %w", *config.Arch, err)
		}

		tag := fmt.Sprintf("%s-%s", tagName, *config.Arch)
		if err := s.Push(ctx, tag, img); err != nil {
			return fmt.Errorf("error pushing image for arch %s: %w", *config.Arch, err)
		}

		fmt.Printf("Successfully built and pushed image for arch %s\n", *config.Arch)

		// Add the descriptor with platform information to the manifests
		manifests = append(manifests, withPlatform(img.Descriptor(), *config.Arch, "linux"))
	}

	// Build index manifest
	index := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:   ocispec.MediaTypeImageIndex,
		Manifests:   manifests,
		Annotations: annotations,
	}

	indexImage, err := imageutil.NewIndexImage(index)
	if err != nil {
		return fmt.Errorf("error creating index image: %w", err)
	}

	if err := s.PushIndexManifest(ctx, indexImage, &index, tagName); err != nil {
		return fmt.Errorf("error pushing index manifest: %w", err)
	}

	fmt.Println("Successfully built multi-arch index:", tagName)
	return nil

}

func withPlatform(desc ocispec.Descriptor, arch, os string) ocispec.Descriptor {
	desc.Platform = &ocispec.Platform{
		Architecture: arch,
		OS:           os,
	}
	desc.MediaType = ocispec.MediaTypeImageManifest
	return desc
}

func buildImage(
	_ context.Context,
	rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath, cmdLinePath *string,
) (image.Image, error) {
	var cmdLineContent string
	if cmdLinePath != nil {
		content, err := os.ReadFile(*cmdLinePath)
		if err != nil {
			return nil, fmt.Errorf("error reading cmdline file: %w", err)
		}
		cmdLineContent = string(content)
	}

	builder := imageutil.NewJSONConfigBuilder(
		&ironcoreimage.Config{CommandLine: cmdLineContent},
		imageutil.WithMediaType(ironcoreimage.ConfigMediaType),
	)

	if rootFSPath != nil {
		builder = builder.FileLayer(*rootFSPath, imageutil.WithMediaType(ironcoreimage.RootFSLayerMediaType))
	}
	if initRAMFSPath != nil {
		builder = builder.FileLayer(*initRAMFSPath, imageutil.WithMediaType(ironcoreimage.InitRAMFSLayerMediaType))
	}
	if kernelPath != nil {
		builder = builder.FileLayer(*kernelPath, imageutil.WithMediaType(ironcoreimage.KernelLayerMediaType))
	}
	if squashFSPath != nil {
		builder = builder.FileLayer(*squashFSPath, imageutil.WithMediaType(ironcoreimage.SquashFSLayerMediaType))
	}
	if ukiPath != nil {
		builder = builder.FileLayer(*ukiPath, imageutil.WithMediaType(ironcoreimage.UKILayerMediaType))
	}
	if isoPath != nil {
		builder = builder.FileLayer(*isoPath, imageutil.WithMediaType(ironcoreimage.ISOLayerMediaType))
	}

	return builder.Complete()
}
