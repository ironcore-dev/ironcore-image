// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

const (
	Kind = "Disk"

	ArtifactType = "application/vnd.ironcore+disk"

	ConfigMediaType = "application/vnd.ironcore.config.disk.v2+json"

	LayerQcow2MediaType = "application/vnd.ironcore.disk.qcow2"
)

type Image struct {
	Manifest ocispec.Manifest
	Config   ocispec.Descriptor
	Chain    []ocispec.Descriptor
}

func (img *Image) GetKind() string {
	return Kind
}

func (img *Image) GetManifest() *ocispec.Manifest {
	return &img.Manifest
}

func (img *Image) GetConfig() ocispec.Descriptor {
	return img.Config
}
