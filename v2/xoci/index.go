// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xoci

import (
	"context"

	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func PackIndex(ctx context.Context, s content.Storage, entries []ocispec.Descriptor) (ocispec.Descriptor, error) {
	index := &ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: entries,
	}

	return xcontent.PushJSON(ctx, s, index, ocispec.MediaTypeImageIndex)
}
