// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ironcoreimage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/remotes"
	"github.com/ironcore-dev/ironcore-image/oci/image"
)

const (
	ConfigMediaType         = "application/vnd.ironcore.image.config.v1+json"
	RootFSLayerMediaType    = "application/vnd.ironcore.image.rootfs"
	InitRAMFSLayerMediaType = "application/vnd.ironcore.image.initramfs"
	KernelLayerMediaType    = "application/vnd.ironcore.image.kernel"
	SquashFSLayerMediaType  = "application/vnd.ironcore.image.squashfs"
	UKILayerMediaType       = "application/vnd.ironcore.image.uki"
	ISOLayerMediaType       = "application/vnd.ironcore.image.iso"

	//TODO: Remove legacy media types support in future versions

	LegacyConfigMediaType         = "application/vnd.ironcore.image.config.v1alpha1+json"
	LegacyRootFSLayerMediaType    = "application/vnd.ironcore.image.rootfs.v1alpha1.rootfs"
	LegacyInitRAMFSLayerMediaType = "application/vnd.ironcore.image.initramfs.v1alpha1.initramfs"
	LegacyKernelLayerMediaType    = "application/vnd.ironcore.image.vmlinuz.v1alpha1.vmlinuz"
	LegacySquashFSLayerMediaType  = "application/vnd.ironcore.image.squashfs.v1alpha1.squashfs"
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

// SetupContext sets up context.Context to not log warnings on ironcore media types.
func SetupContext(ctx context.Context) context.Context {
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, ConfigMediaType, "config-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, RootFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, InitRAMFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, KernelLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, SquashFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, UKILayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, ISOLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, LegacyConfigMediaType, "config-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, LegacyRootFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, LegacyInitRAMFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, LegacyKernelLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, LegacySquashFSLayerMediaType, "layer-")
	return ctx
}

// ResolveImage resolves an oci image to an ironcore Image.
func ResolveImage(ctx context.Context, ociImg image.Image) (*Image, error) {
	ctx = SetupContext(ctx)

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
		case SquashFSLayerMediaType:
			img.SquashFS = layer
		case UKILayerMediaType:
			img.UKI = layer
		case ISOLayerMediaType:
			img.ISO = layer
		case LegacyInitRAMFSLayerMediaType:
			if img.InitRAMFs == nil {
				img.InitRAMFs = layer
			}
		case LegacyKernelLayerMediaType:
			if img.Kernel == nil {
				img.Kernel = layer
			}
		case LegacyRootFSLayerMediaType:
			if img.RootFS == nil {
				img.RootFS = layer
			}
		case LegacySquashFSLayerMediaType:
			if img.SquashFS == nil {
				img.SquashFS = layer
			}
		default:
			return nil, fmt.Errorf("unknown layer type %q", layer.Descriptor().MediaType)
		}
	}
	return &img, nil
}

// Image is an ironcore image.
type Image struct {
	// Config holds additional configuration for a machine / machine pool using the image.
	Config Config
	// RootFS is the layer containing the root file system.
	RootFS image.Layer
	// SquashFS is the layer containing the root file system.
	SquashFS image.Layer
	// InitRAMFs is the layer containing the initramfs / initrd.
	InitRAMFs image.Layer
	// Kernel is the layer containing the kernel.
	Kernel image.Layer
	// UKI is a Unified Kernel Image layer.
	UKI image.Layer
	// ISO is a layer containing a bootable ISO image.
	ISO image.Layer
}
