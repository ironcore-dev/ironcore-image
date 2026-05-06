// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xcontent

import (
	"context"
	"fmt"
	"io"

	"github.com/ironcore-dev/ironcore-image/v2/xio"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func NewDescriptorFromSource(mediaType string, src xio.Source) (ocispec.Descriptor, error) {
	size, err := src.Size()
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("getting size: %w", err)
	}

	rd, err := src.Open()
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("opening: %w", err)
	}
	defer func() { _ = rd.Close() }()

	d, err := digest.FromReader(rd)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("computing digest: %w", err)
	}

	return ocispec.Descriptor{
		MediaType: mediaType,
		Size:      size,
		Digest:    d,
	}, nil
}

type descriptorSource struct {
	ctx     context.Context
	fetcher content.Fetcher
	desc    ocispec.Descriptor
}

func FetchSource(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) xio.Source {
	return &descriptorSource{
		ctx:     ctx,
		fetcher: fetcher,
		desc:    desc,
	}
}

func (s *descriptorSource) Open() (io.ReadCloser, error) {
	return s.fetcher.Fetch(s.ctx, s.desc)
}

func (s *descriptorSource) Size() (int64, error) {
	return s.desc.Size, nil
}
