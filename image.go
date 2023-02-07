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

package onmetal_image

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onmetal/onmetal-image/oci/image"
)

const (
	ConfigMediaType         = "application/vnd.onmetal.image.config.v1alpha1+json"
	RootFSLayerMediaType    = "application/vnd.onmetal.image.rootfs.v1alpha1.rootfs"
	InitRAMFSLayerMediaType = "application/vnd.onmetal.image.initramfs.v1alpha1.initramfs"
	KernelLayerMediaType    = "application/vnd.onmetal.image.vmlinuz.v1alpha1.vmlinuz"
)

type Config struct {
	CommandLine string `json:"commandLine,omitempty"`
}

func readImageConfig(ctx context.Context, img image.Image) (*Config, error) {
	configLayer, err := img.Config(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config layer: %w", err)
	}

	rc, err := configLayer.Content(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// TODO: Parse Config depending on configLayer.Descriptor().MediaType
	config := &Config{}
	if err := json.NewDecoder(rc).Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}
	return config, nil
}

// ResolveImage resolves an oci image to an onmetal Image.
func ResolveImage(ctx context.Context, ociImg image.Image) (*Image, error) {
	config, err := readImageConfig(ctx, ociImg)
	if err != nil {
		return nil, err
	}

	layers, err := ociImg.Layers(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting image layers: %w", err)
	}

	img := Image{Config: *config}
	for _, layer := range layers {
		switch layer.Descriptor().MediaType {
		case InitRAMFSLayerMediaType:
			img.InitRAMFs = layer
		case KernelLayerMediaType:
			img.Kernel = layer
		case RootFSLayerMediaType:
			img.RootFS = layer
		default:
			return nil, fmt.Errorf("unknown layer type %q", layer.Descriptor().MediaType)
		}
	}
	var missing []string
	if img.RootFS == nil {
		missing = append(missing, "rootfs")
	}
	if img.Kernel == nil {
		missing = append(missing, "kernel")
	}
	if img.InitRAMFs == nil {
		missing = append(missing, "initramfs")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("incomplete image: components are missing: %v", missing)
	}

	return &img, nil
}

// Image is an onmetal image.
type Image struct {
	// Config holds additional configuration for a machine / machine pool using the image.
	Config Config
	// RootFS is the layer containing the root file system.
	RootFS image.Layer
	// InitRAMFs is the layer containing the initramfs / initrd.
	InitRAMFs image.Layer
	// Kernel is the layer containing the kernel.
	Kernel image.Layer
}
