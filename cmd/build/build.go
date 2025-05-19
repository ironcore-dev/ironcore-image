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
	"github.com/ironcore-dev/ironcore-image/oci/imageutil"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v2"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/spf13/cobra"
)

type Config struct {
	Architectures map[string]struct {
		RootFS    string `yaml:"rootfs"`
		Kernel    string `yaml:"kernel"`
		SquashFS  string `yaml:"squashfs,omitempty"`
		InitRAMFS string `yaml:"initramfs,omitempty"`
		UKI       string `yaml:"uki,omitempty"`
		ISO       string `yaml:"iso,omitempty"`
	} `yaml:"architectures"`
}

func Command(storeFactory common.StoreFactory) *cobra.Command {
	var (
		tagName string

		multiArch  bool
		configPath string

		rootFSPath    string
		squashFSPath  string
		initRAMFSPath string
		kernelPath    string
		ukiPath       string
		isoPath       string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image and store it to the local store with an optional tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, storeFactory, tagName, multiArch, configPath,
				rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	cmd.Flags().StringVar(&tagName, "tag", "", "Optional tag of image.")
	cmd.Flags().BoolVar(&multiArch, "multi-arch", false, "Enable multi-architecture (amd64 + arm64) image build.")
	cmd.Flags().StringVar(&rootFSPath, "rootfs-file", "", "Path to root fs file.")
	cmd.Flags().StringVar(&squashFSPath, "squashfs-file", "", "Path to squash fs file.")
	cmd.Flags().StringVar(&initRAMFSPath, "initramfs-file", "", "Path to initramfs file.")
	cmd.Flags().StringVar(&kernelPath, "kernel-file", "", "Path to kernel file.")
	cmd.Flags().StringVar(&ukiPath, "uki-file", "", "Path to Unified Kernel Image (UKI) file.")
	cmd.Flags().StringVar(&isoPath, "iso-file", "", "Path to bootable ISO image file.")

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	tagName string,
	multiArch bool,
	configPath string,
	rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath string,
) error {
	var rootFSPathAMD64, squashFSPathAMD64, initRAMFSPathAMD64, kernelPathAMD64, ukiPathAMD64, isoPathAMD64 string
	var rootFSPathARM64, squashFSPathARM64, initRAMFSPathARM64, kernelPathARM64, ukiPathARM64, isoPathARM64 string
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	if multiArch {
		if configPath != "" {
			file, err := os.Open(configPath)
			if err != nil {
				return fmt.Errorf("error opening config file: %w", err)
			}
			defer func() {
				if cerr := file.Close(); cerr != nil {
					fmt.Printf("error closing config file: %v\n", cerr)
				}
			}()

			var config Config
			if err := yaml.NewDecoder(file).Decode(&config); err != nil {
				return fmt.Errorf("error parsing config file: %w", err)
			}

			// Populate paths from the config
			if archConfig, ok := config.Architectures["amd64"]; ok {
				rootFSPathAMD64 = archConfig.RootFS
				kernelPathAMD64 = archConfig.Kernel
				squashFSPathAMD64 = archConfig.SquashFS
				initRAMFSPathAMD64 = archConfig.InitRAMFS
				ukiPathAMD64 = archConfig.UKI
				isoPathAMD64 = archConfig.ISO
			}
			if archConfig, ok := config.Architectures["arm64"]; ok {
				rootFSPathARM64 = archConfig.RootFS
				kernelPathARM64 = archConfig.Kernel
				squashFSPathARM64 = archConfig.SquashFS
				initRAMFSPathARM64 = archConfig.InitRAMFS
				ukiPathARM64 = archConfig.UKI
				isoPathARM64 = archConfig.ISO
			}
		} else {
			if rootFSPath != "" {
				rootFSPathAMD64, rootFSPathARM64, err = parseMultiArchPaths(strings.Split(rootFSPath, ","), "rootfs-file")
				if err != nil {
					return err
				}
			}

			if kernelPath != "" {
				kernelPathAMD64, kernelPathARM64, err = parseMultiArchPaths(strings.Split(kernelPath, ","), "kernel-file")
				if err != nil {
					return err
				}
			}

			if squashFSPath != "" {
				squashFSPathAMD64, squashFSPathARM64, err = parseMultiArchPaths(strings.Split(squashFSPath, ","), "squashfs-file")
				if err != nil {
					return err
				}
			}

			if ukiPath != "" {
				ukiPathAMD64, ukiPathARM64, err = parseMultiArchPaths(strings.Split(ukiPath, ","), "uki-file")
				if err != nil {
					return err
				}
			}

			if initRAMFSPath != "" {
				initRAMFSPathAMD64, initRAMFSPathARM64, err = parseMultiArchPaths(strings.Split(initRAMFSPath, ","), "initramfs-file")
				if err != nil {
					return err
				}
			}

			if isoPath != "" {
				isoPathAMD64, isoPathARM64, err = parseMultiArchPaths(strings.Split(isoPath, ","), "iso-file")
				if err != nil {
					return err
				}
			}
		}

		// Build AMD64 image
		amd64Img, err := buildImage(ctx, rootFSPathAMD64, squashFSPathAMD64, initRAMFSPathAMD64, kernelPathAMD64, ukiPathAMD64, isoPathAMD64)
		if err != nil {
			return fmt.Errorf("error building amd64 image: %w", err)
		}

		// Build ARM64 image
		arm64Img, err := buildImage(ctx, rootFSPathARM64, squashFSPathARM64, initRAMFSPathARM64, kernelPathARM64, ukiPathARM64, isoPathARM64)
		if err != nil {
			return fmt.Errorf("error building arm64 image: %w", err)
		}

		// Push AMD64 and ARM64 images first to the store
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

	img, err := buildImage(ctx, rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath)
	if err != nil {
		return fmt.Errorf("error building image: %w", err)
	}

	if err := s.Push(ctx, tagName, img); err != nil {
		return fmt.Errorf("error pushing image: %w", err)
	}

	fmt.Println("Successfully built", tagName)
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
	_ context.Context,
	rootFSPath, squashFSPath, initRAMFSPath, kernelPath, ukiPath, isoPath string,
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

// Helper function to parse multi-arch paths
func parseMultiArchPaths(paths []string, flagName string) (string, string, error) {
	var amd64Path, arm64Path string
	for _, p := range paths {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid format for --%s, expected 'arch=path'", flagName)
		}
		switch parts[0] {
		case "amd64":
			amd64Path = parts[1]
		case "arm64":
			arm64Path = parts[1]
		default:
			return "", "", fmt.Errorf("unsupported architecture: %s", parts[0])
		}
	}
	return amd64Path, arm64Path, nil
}
