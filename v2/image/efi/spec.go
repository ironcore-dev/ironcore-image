// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package efi

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

const (
	Kind = "EFI"

	ArtifactType = "application/vnd.ironcore+efi"

	ConfigMediaType = "application/vnd.ironcore.config.efi.v2+json"

	LayerEFIExecutableMediaType = "application/vnd.ironcore.executable.efi"
)

type Image struct {
	Manifest   ocispec.Manifest
	Config     ocispec.Descriptor
	Executable ocispec.Descriptor
}

func (img *Image) GetManifest() *ocispec.Manifest {
	return &img.Manifest
}
