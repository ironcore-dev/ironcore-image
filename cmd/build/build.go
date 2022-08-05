// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package build

import (
	"context"
	"fmt"

	"github.com/onmetal/onmetal-image/cmd/common"

	onmetalimage "github.com/onmetal/onmetal-image"

	"github.com/onmetal/onmetal-image/oci/imageutil"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	var (
		tagName       string
		rootFSPath    string
		initRAMFSPath string
		kernelPath    string
		commandLine   string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image and store it to the local store with an optional tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, storeFactory, tagName, rootFSPath, initRAMFSPath, kernelPath, commandLine)
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Optional tag of image.")
	cmd.Flags().StringVar(&rootFSPath, "rootfs-file", "", "Path pointing to a root fs file.")
	cmd.Flags().StringVar(&initRAMFSPath, "initramfs-file", "", "Path pointing to an initram fs file.")
	cmd.Flags().StringVar(&kernelPath, "kernel-file", "", "Path pointing to a kernel file (usually ending with 'vmlinuz').")
	cmd.Flags().StringVar(&commandLine, "command-line", "", "Command line arguments to supply to the kernel.")

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	ref, rootFSPath, initRAMFSPath, kernelPath, commandLine string,
) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	img, err :=
		imageutil.NewJSONConfigBuilder(
			&onmetalimage.Config{CommandLine: commandLine},
			imageutil.WithMediaType(onmetalimage.ConfigMediaType),
		).
			FileLayer(rootFSPath, imageutil.WithMediaType(onmetalimage.RootFSLayerMediaType)).
			FileLayer(initRAMFSPath, imageutil.WithMediaType(onmetalimage.InitRAMFSLayerMediaType)).
			FileLayer(kernelPath, imageutil.WithMediaType(onmetalimage.KernelLayerMediaType)).
			Complete()
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
