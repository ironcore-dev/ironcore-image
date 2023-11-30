// Copyright 2021 IronCore authors
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

	ironcoreimage "github.com/ironcore-dev/ironcore-image"
	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

type LayerType string

const (
	Kernel    LayerType = "kernel"
	RootFS    LayerType = "rootfs"
	InitRAMFS LayerType = "initramfs"
)

func Command(requestResolverFactory common.RequestResolverFactory) *cobra.Command {
	var layer LayerType
	cmd := &cobra.Command{
		Use:   "url image[:tag] [layer-media-type]",
		Short: "Compute the URL for retrieving a remote image manifest or a layer.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := args[0]
			return Run(ctx, requestResolverFactory, ref, layer)
		},
	}
	cmd.Flags().StringVar((*string)(&layer), "layer", "", "Specify to get the URL to a specific layer.")

	return cmd
}

var layerTypeToMediaType = map[LayerType]string{
	RootFS:    ironcoreimage.RootFSLayerMediaType,
	InitRAMFS: ironcoreimage.InitRAMFSLayerMediaType,
	Kernel:    ironcoreimage.KernelLayerMediaType,
}

func Run(ctx context.Context, requestResolverFactory common.RequestResolverFactory, ref string, layer LayerType) error {
	resolver, err := requestResolverFactory()
	if err != nil {
		return fmt.Errorf("error creating request resolver: %w", err)
	}

	info, err := resolver.Resolve(ctx, ref)
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
