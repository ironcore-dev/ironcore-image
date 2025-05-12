// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/opencontainers/image-spec/specs-go"

	ironcoreimage "github.com/ironcore-dev/ironcore-image"
	"github.com/ironcore-dev/ironcore-image/oci/imageutil"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	var (
		tagName string

		arch      string
		multiArch bool

		rootFSPathAMD64    string
		squashFSPathAMD64  string
		initRAMFSPathAMD64 string
		kernelPathAMD64    string
		ukiPathAMD64       string
		isoPathAMD64       string

		rootFSPathARM64    string
		squashFSPathARM64  string
		initRAMFSPathARM64 string
		kernelPathARM64    string
		ukiPathARM64       string
		isoPathARM64       string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image and store it to the local store with an optional tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, storeFactory, tagName, arch, multiArch,
				rootFSPathAMD64, squashFSPathAMD64, initRAMFSPathAMD64, kernelPathAMD64, ukiPathAMD64, isoPathAMD64,
				rootFSPathARM64, squashFSPathARM64, initRAMFSPathARM64, kernelPathARM64, ukiPathARM64, isoPathARM64)
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Optional tag of image.")
	cmd.Flags().StringVar(&arch, "arch", "", "Target architecture (amd64 or arm64). Cannot be used with --multi-arch.")
	cmd.Flags().BoolVar(&multiArch, "multi-arch", false, "Enable multi-architecture (amd64 + arm64) image build.")

	cmd.Flags().StringVar(&rootFSPathAMD64, "rootfs-file-amd64", "", "Path to AMD64 root fs file.")
	cmd.Flags().StringVar(&squashFSPathAMD64, "squashfs-file-amd64", "", "Path to AMD64 squash fs file.")
	cmd.Flags().StringVar(&initRAMFSPathAMD64, "initramfs-file-amd64", "", "Path to AMD64 initramfs file.")
	cmd.Flags().StringVar(&kernelPathAMD64, "kernel-file-amd64", "", "Path to AMD64 kernel file.")
	cmd.Flags().StringVar(&ukiPathAMD64, "uki-file-amd64", "", "Path to AMD64 Unified Kernel Image (UKI) file.")
	cmd.Flags().StringVar(&isoPathAMD64, "iso-file-amd64", "", "Path to AMD64 bootable ISO image file.")

	cmd.Flags().StringVar(&rootFSPathARM64, "rootfs-file-arm64", "", "Path to ARM64 root fs file.")
	cmd.Flags().StringVar(&squashFSPathARM64, "squashfs-file-arm64", "", "Path to ARM64 squash fs file.")
	cmd.Flags().StringVar(&initRAMFSPathARM64, "initramfs-file-arm64", "", "Path to ARM64 initramfs file.")
	cmd.Flags().StringVar(&kernelPathARM64, "kernel-file-arm64", "", "Path to ARM64 kernel file.")
	cmd.Flags().StringVar(&ukiPathARM64, "uki-file-arm64", "", "Path to ARM64 Unified Kernel Image (UKI) file.")
	cmd.Flags().StringVar(&isoPathARM64, "iso-file-arm64", "", "Path to ARM64 bootable ISO image file.")

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	tagName string,
	arch string,
	multiArch bool,
	rootFSPathAMD64, squashFSPathAMD64, initRAMFSPathAMD64, kernelPathAMD64, ukiPathAMD64, isoPathAMD64 string,
	rootFSPathARM64, squashFSPathARM64, initRAMFSPathARM64, kernelPathARM64, ukiPathARM64, isoPathARM64 string,
) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	if multiArch {
		// TODO: Fix this with a better solution
		if rootFSPathAMD64 == "" || kernelPathAMD64 == "" || rootFSPathARM64 == "" || kernelPathARM64 == "" {
			return fmt.Errorf("multi-arch build requires all amd64 and arm64 paths to be provided")
		}

		// Build AMD64 image
		amd64Img, err := buildImage(ctx, rootFSPathAMD64, squashFSPathAMD64, initRAMFSPathAMD64, kernelPathAMD64, ukiPathAMD64, isoPathAMD64, "amd64")
		if err != nil {
			return fmt.Errorf("error building amd64 image: %w", err)
		}

		// Build ARM64 image
		arm64Img, err := buildImage(ctx, rootFSPathARM64, squashFSPathARM64, initRAMFSPathARM64, kernelPathARM64, ukiPathARM64, isoPathARM64, "arm64")
		if err != nil {
			return fmt.Errorf("error building arm64 image: %w", err)
		}

		// Push AMD64 and ARM64 images first
		if err := s.Push(ctx, tagName+"-amd64", amd64Img); err != nil {
			return fmt.Errorf("error pushing amd64 image: %w", err)
		}
		if err := s.Push(ctx, tagName+"-arm64", arm64Img); err != nil {
			return fmt.Errorf("error pushing arm64 image: %w", err)
		}

		fmt.Println("Successfully built AMD64 and ARM64 images.")

		// Build index manifest
		index := ocispec.Index{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: []ocispec.Descriptor{
				withPlatform(amd64Img.Descriptor(), "amd64", "linux"),
				withPlatform(arm64Img.Descriptor(), "arm64", "linux"),
			},
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

	// Single arch build
	var rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath string

	switch arch {
	case "amd64":
		rootFSPath = rootFSPathAMD64
		squashFSPath = squashFSPathAMD64
		initRAMFSPath = initRAMFSPathAMD64
		kernelPath = kernelPathAMD64
		ukiPath = ukiPathAMD64
		isoPath = isoPathAMD64
	case "arm64":
		rootFSPath = rootFSPathARM64
		squashFSPath = squashFSPathARM64
		initRAMFSPath = initRAMFSPathARM64
		kernelPath = kernelPathARM64
		ukiPath = ukiPathARM64
		isoPath = isoPathARM64
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	img, err := buildImage(ctx, rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath, arch)
	if err != nil {
		return fmt.Errorf("error building image: %w", err)
	}

	if err := s.Push(ctx, tagName, img); err != nil {
		return fmt.Errorf("error pushing image: %w", err)
	}

	fmt.Println("Successfully built and pushed", tagName)
	return nil
}

func withPlatform(desc ocispec.Descriptor, arch, os string) ocispec.Descriptor {
	desc.Platform = &ocispec.Platform{
		Architecture: arch,
		OS:           os,
	}
	return desc
}

func buildImage(
	ctx context.Context,
	rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath, arch string,
) (image.Image, error) {
	builder := imageutil.NewJSONConfigBuilder(
		&ironcoreimage.Config{},
		imageutil.WithMediaType(ironcoreimage.ConfigMediaType),
	)

	if rootFSPath != "" {
		builder = builder.FileLayer(rootFSPath, imageutil.WithMediaType(ironcoreimage.RootFSLayerMediaType))
	}
	if initRAMFSPath != "" {
		builder = builder.FileLayer(initRAMFSPath, imageutil.WithMediaType(ironcoreimage.InitRAMFSLayerMediaType))
	}
	if kernelPath != "" {
		builder = builder.FileLayer(kernelPath, imageutil.WithMediaType(ironcoreimage.KernelLayerMediaType))
	}
	if squashFSPath != "" {
		builder = builder.FileLayer(squashFSPath, imageutil.WithMediaType(ironcoreimage.SquashFSLayerMediaType))
	}
	if ukiPath != "" {
		builder = builder.FileLayer(ukiPath, imageutil.WithMediaType(ironcoreimage.UKILayerMediaType))
	}
	if isoPath != "" {
		builder = builder.FileLayer(isoPath, imageutil.WithMediaType(ironcoreimage.ISOLayerMediaType))
	}

	return builder.Complete()
}
