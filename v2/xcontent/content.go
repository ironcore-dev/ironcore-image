// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xcontent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ironcore-dev/ironcore-image/v2/xio"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	orascontent "oras.land/oras-go/v2/content"
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

func FetchAllJSON(ctx context.Context, fetcher orascontent.Fetcher, desc ocispec.Descriptor, v any) error {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("fetching content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	dec := json.NewDecoder(rc)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decoding content: %w", err)
	}
	return nil
}

func FetchCopy(ctx context.Context, fetcher orascontent.Fetcher, desc ocispec.Descriptor, dst io.Writer) error {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("fetching content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	_, err = io.Copy(dst, rc)
	return err
}

func FetchWriteFile(ctx context.Context, fetcher orascontent.Fetcher, desc ocispec.Descriptor, name string, perm os.FileMode) error {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("fetching content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, rc); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return fmt.Errorf("writing content: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("closing file: %w", err)
	}
	return nil
}
