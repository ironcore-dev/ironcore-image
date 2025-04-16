// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	ironcoreimage "github.com/ironcore-dev/ironcore-image"
	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/imageutil"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	var (
		tagName       string
		rootFSPath    string
		squashFSPath  string
		initRAMFSPath string
		kernelPath    string
		ukiPath       string
		isoPath       string
		commandLine   string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image and store it to the local store with an optional tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, storeFactory, tagName, rootFSPath, squashFSPath, initRAMFSPath, kernelPath, commandLine, ukiPath, isoPath)
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Optional tag of image.")
	cmd.Flags().StringVar(&rootFSPath, "rootfs-file", "", "Path pointing to a root fs file.")
	cmd.Flags().StringVar(&squashFSPath, "squashfs-file", "", "Path pointing to a squash fs file.")
	cmd.Flags().StringVar(&initRAMFSPath, "initramfs-file", "", "Path pointing to an initram fs file.")
	cmd.Flags().StringVar(&kernelPath, "kernel-file", "", "Path pointing to a kernel file (usually ending with 'vmlinuz').")
	cmd.Flags().StringVar(&commandLine, "command-line", "", "Command line arguments to supply to the kernel.")
	cmd.Flags().StringVar(&ukiPath, "uki-file", "", "Optional path to a Unified Kernel Image (UKI) file.")
	cmd.Flags().StringVar(&isoPath, "iso-file", "", "Optional path to a bootable ISO image file.")

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	ref, rootFSPath, squashFSPath, initRAMFSPath, kernelPath, commandLine, ukiPath, isoPath string,
) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	builder := imageutil.NewJSONConfigBuilder(
		&ironcoreimage.Config{CommandLine: commandLine},
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

	img, err := builder.Complete()
	if err != nil {
		return fmt.Errorf("error building image: %w", err)
	}

	if ref != "" {
		if err := s.Push(ctx, ref, img); err != nil {
			return fmt.Errorf("error pushing to ref %s: %w", ref, err)
		}
		fmt.Println("Successfully built", ref, img.Descriptor().Digest.Encoded())
	} else {
		if err := s.Put(ctx, img); err != nil {
			return fmt.Errorf("error putting image: %w", err)
		}
		fmt.Println("Successfully built", img.Descriptor().Digest.Encoded())
	}
	return nil
}
