// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

type cacheKey struct {
	MediaType string
	Digest    digest.Digest
	Size      int64
}

func cacheKeyFromDescriptor(desc ocispec.Descriptor) cacheKey {
	return cacheKey{
		MediaType: desc.MediaType,
		Digest:    desc.Digest,
		Size:      desc.Size,
	}
}

type contentCache struct {
	content sync.Map
	limit   int64
}

func (c *contentCache) Fetch(ctx context.Context, storage content.ReadOnlyStorage, desc ocispec.Descriptor) (io.ReadCloser, error) {
	if _, ok := metadataMediaTypes[desc.MediaType]; !ok {
		return storage.Fetch(ctx, desc)
	}

	if desc.Size > c.limit {
		return nil, fmt.Errorf(
			"content size %v exceeds cache size limit %v: %w",
			desc.Size,
			c.limit,
			errdef.ErrSizeExceedsLimit)
	}

	key := cacheKeyFromDescriptor(desc)
	if data, ok := c.content.Load(key); ok {
		return io.NopCloser(bytes.NewReader(data.([]byte))), nil
	}

	rc, err := storage.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	r := &onEOFReader{
		r: io.TeeReader(io.LimitReader(rc, c.limit), &buf),
		f: func() {
			if buf.Len() == int(desc.Size) {
				c.content.LoadOrStore(key, buf.Bytes())
			}
		},
	}

	return struct {
		io.Reader
		io.Closer
	}{
		r,
		rc,
	}, nil
}

func (c *contentCache) Exists(ctx context.Context, storage content.ReadOnlyStorage, desc ocispec.Descriptor) (bool, error) {
	if _, ok := metadataMediaTypes[desc.MediaType]; !ok {
		return storage.Exists(ctx, desc)
	}
	if _, ok := c.content.Load(cacheKeyFromDescriptor(desc)); ok {
		return true, nil
	}
	return storage.Exists(ctx, desc)
}

type metadataCacheReadOnlyTarget struct {
	cache  contentCache
	target oras.ReadOnlyTarget
}

func (c *metadataCacheReadOnlyTarget) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return c.cache.Fetch(ctx, c.target, target)
}

func (c *metadataCacheReadOnlyTarget) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return c.cache.Exists(ctx, c.target, target)
}

func (c *metadataCacheReadOnlyTarget) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	return c.target.Resolve(ctx, reference)
}

var (
	metadataMediaTypes = map[string]struct{}{
		ocispec.MediaTypeImageIndex:    {},
		ocispec.MediaTypeImageManifest: {},
	}
)

func cacheReadOnlyTarget(target oras.ReadOnlyTarget, limit int64) oras.ReadOnlyTarget {
	switch storage := target.(type) {
	case *metadataCacheReadOnlyTarget:
		if storage.cache.limit == limit {
			return storage
		}
	}
	return &metadataCacheReadOnlyTarget{
		cache: contentCache{
			limit: limit,
		},
		target: target,
	}
}

type onEOFReader struct {
	r   io.Reader
	eof bool
	f   func()
}

func (r *onEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	if errors.Is(err, io.EOF) && !r.eof {
		r.eof = true
		r.f()
	}
	return n, err
}
