// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package uki

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

const (
	Kind = "UKI"

	ArtifactType = "application/vnd.ironcore+uki"

	MediaTypeConfig = "application/vnd.ironcore.config.uki.v2+json"

	MediaTypeLayerKernel = "application/vnd.ironcore.kernel.efi"
	MediaTypeLayerStub   = "application/vnd.ironcore.stub.efi"

	MediaTypeLayerInitrd     = "application/vnd.ironcore.initrd.cpio"
	MediaTypeLayerInitrdGzip = MediaTypeLayerInitrd + "+gzip"
	MediaTypeLayerInitrdZstd = MediaTypeLayerInitrd + "+zstd"
	MediaTypeLayerInitrdXz   = MediaTypeLayerInitrd + "+xz"
	MediaTypeLayerInitrdLz4  = MediaTypeLayerInitrd + "+lz4"
)

var InitrdLayerMediaTypes = []string{
	MediaTypeLayerInitrd,
	MediaTypeLayerInitrdGzip,
	MediaTypeLayerInitrdZstd,
	MediaTypeLayerInitrdXz,
	MediaTypeLayerInitrdLz4,
}

type Image struct {
	Manifest ocispec.Manifest
	Config   ocispec.Descriptor

	Kernel  ocispec.Descriptor
	Initrds []ocispec.Descriptor
	Stub    ocispec.Descriptor
}

func (img *Image) GetManifest() *ocispec.Manifest {
	return &img.Manifest
}

func (img *Image) GetConfig() ocispec.Descriptor {
	return img.Config
}
