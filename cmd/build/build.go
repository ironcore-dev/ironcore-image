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

	"github.com/distribution/distribution/reference"

	"github.com/onmetal/onmetal-image/client"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var (
		tagName       string
		rootFSPath    string
		initRAMFSPath string
		vmlinuzPath   string
	)

	cmd := &cobra.Command{
		Use: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, tagName, rootFSPath, initRAMFSPath, vmlinuzPath)
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Optional tag of image.")
	cmd.Flags().StringVar(&rootFSPath, "rootfs-file", "", "Path pointing to a root fs file.")
	cmd.Flags().StringVar(&initRAMFSPath, "initramfs-file", "", "Path pointing to an initram fs file.")
	cmd.Flags().StringVar(&vmlinuzPath, "vmlinuz-file", "", "Path pointing to a kernel file (usually ending with 'vmlinuz').")

	return cmd
}

func Run(ctx context.Context, tagName, rootFSPath, initRAMFSPath, vmlinuzPath string) error {
	var ref reference.Named
	if tagName != "" {
		var err error
		ref, err = reference.ParseNamed(tagName)
		if err != nil {
			return err
		}
	}

	c, err := client.New()
	if err != nil {
		return fmt.Errorf("could not create client: %w", err)
	}

	built, err := c.Build(ctx, rootFSPath, initRAMFSPath, vmlinuzPath, &client.BuildOptions{Reference: ref})
	if err != nil {
		return fmt.Errorf("error building image: %w", err)
	}

	if ref != nil {
		fmt.Println("Successfully built", ref, built.Digest.Encoded())
	} else {
		fmt.Println("Successfully built", built.Digest.Encoded())
	}
	return nil
}
