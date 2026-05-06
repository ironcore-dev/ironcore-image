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

type metadataProxy struct {
	storage content.ReadOnlyStorage
	content sync.Map // map[cacheKey][]byte
	limit   int64
}

var (
	metadataMediaTypes = map[string]struct{}{
		ocispec.MediaTypeImageIndex:    {},
		ocispec.MediaTypeImageManifest: {},
	}
)

func newMetadataProxy(storage content.ReadOnlyStorage, limit int64) *metadataProxy {
	return &metadataProxy{
		storage: storage,
		limit:   limit,
	}
}

func (p *metadataProxy) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	if _, ok := metadataMediaTypes[desc.MediaType]; !ok {
		return p.storage.Fetch(ctx, desc)
	}

	if desc.Size > p.limit {
		return nil, fmt.Errorf(
			"content size %v exceeds cache size limit %v: %w",
			desc.Size,
			p.limit,
			errdef.ErrSizeExceedsLimit)
	}

	key := cacheKeyFromDescriptor(desc)
	if data, ok := p.content.Load(key); ok {
		return io.NopCloser(bytes.NewReader(data.([]byte))), nil
	}

	rc, err := p.storage.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	r := &onEOFReader{
		r: io.TeeReader(io.LimitReader(rc, p.limit), &buf),
		f: func() {
			if buf.Len() == int(desc.Size) {
				p.content.LoadOrStore(key, buf.Bytes())
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

func (p *metadataProxy) Exists(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	if _, ok := metadataMediaTypes[desc.MediaType]; !ok {
		return p.storage.Exists(ctx, desc)
	}
	if _, ok := p.content.Load(cacheKeyFromDescriptor(desc)); ok {
		return true, nil
	}
	return p.storage.Exists(ctx, desc)
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
