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

package url

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/onmetal/onmetal-image/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	onmetalimage "github.com/onmetal/onmetal-image"

	"github.com/onmetal/onmetal-image/cmd/common"

	"github.com/spf13/cobra"
)

type LayerType string

const (
	Kernel    LayerType = "kernel"
	RootFS    LayerType = "rootfs"
	InitRAMFS LayerType = "initramfs"
)

func Command(urlerFactory common.URLerFactory) *cobra.Command {
	var layer LayerType
	cmd := &cobra.Command{
		Use:  "url image[:tag] [layer-media-type]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := args[0]
			return Run(ctx, urlerFactory, ref, layer)
		},
	}
	cmd.Flags().StringVar((*string)(&layer), "layer", "", "Specify to get the URL to a specific layer.")

	return cmd
}

var layerTypeToMediaType = map[LayerType]string{
	RootFS:    onmetalimage.RootFSLayerMediaType,
	InitRAMFS: onmetalimage.InitRAMFSLayerMediaType,
	Kernel:    onmetalimage.KernelLayerMediaType,
}

func Run(ctx context.Context, urlerFactory common.URLerFactory, ref string, layer LayerType) error {
	u, err := urlerFactory()
	if err != nil {
		return fmt.Errorf("error creating urler: %w", err)
	}

	info, err := u.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("error resolving ref %s: %w", ref, err)
	}

	var request docker.Request
	if layer != "" {
		manifest, err := info.Manifest(ctx)
		if err != nil {
			return fmt.Errorf("could not resolve manifest: %w", err)
		}

		var (
			mediaType = layerTypeToMediaType[layer]
			desc      *ocispec.Descriptor
		)
		for _, layer := range manifest.Layers {
			if layer.MediaType == mediaType {
				layer := layer
				desc = &layer
				break
			}
		}
		if desc == nil {
			return fmt.Errorf("no layer with type %s found", layer)
		}

		layerInfo, err := info.Layer(ctx, *desc)
		if err != nil {
			return fmt.Errorf("could not lookup layer %s: %w", desc.Digest, err)
		}

		request = layerInfo.Request()
	} else {
		request = info.Request()
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(request)
}
