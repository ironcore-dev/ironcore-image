// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Store is a storage backed by the local file system.
type Store struct {
	root  string
	store content.Store
}

// NewStore returns a new Store.
func NewStore(root string) (*Store, error) {
	s, err := local.NewStore(root)
	if err != nil {
		return nil, err
	}
	return &Store{
		root:  root,
		store: s,
	}, nil
}

// Info implements content.Store.
func (s *Store) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	return s.store.Info(ctx, dgst)
}

// BlobPath returns the path where the blob of the digest is located at.
//
// The blob path may not exist, and it is up to the caller to validate the file has the expected content.
func (s *Store) BlobPath(dgst digest.Digest) (string, error) {
	if err := dgst.Validate(); err != nil {
		return "", fmt.Errorf("cannot calculate blob path from invalid digest: %v: %w", err, errdefs.ErrInvalidArgument)
	}

	return filepath.Join(s.root, "blobs", dgst.Algorithm().String(), dgst.Hex()), nil
}

// Update implements content.Store.
func (s *Store) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return s.store.Update(ctx, info, fieldpaths...)
}

// Walk implements content.Store.
func (s *Store) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return s.store.Walk(ctx, fn, filters...)
}

// Delete implements content.Store.
func (s *Store) Delete(ctx context.Context, dgst digest.Digest) error {
	return s.store.Delete(ctx, dgst)
}

// ReaderAt implements content.Store.
func (s *Store) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	return s.store.ReaderAt(ctx, desc)
}

// Status implements content.Store.
func (s *Store) Status(ctx context.Context, ref string) (content.Status, error) {
	return s.store.Status(ctx, ref)
}

// ListStatuses implements content.Store.
func (s *Store) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return s.store.ListStatuses(ctx, filters...)
}

// Abort implements content.Store.
func (s *Store) Abort(ctx context.Context, ref string) error {
	return s.store.Abort(ctx, ref)
}

// Writer implements content.Store.
func (s *Store) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return s.store.Writer(ctx, opts...)
}
