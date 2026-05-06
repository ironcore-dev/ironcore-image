package image

import (
	"context"
	"errors"
	"iter"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

// CopyGraphOptions are options for the CopyGraph operation.
type CopyGraphOptions struct {
	// Concurrency limits the maximum number of concurrent copy tasks.
	// If less than or equal to 0, a default (currently 3) is used.
	Concurrency int
	// FindSuccessors finds the successors of the current node.
	// fetcher provides cached access to the source storage, and is suitable
	// for fetching non-leaf nodes like manifests. Since anything fetched from
	// fetcher will be cached in the memory, it is recommended to use original
	// source storage to fetch large blobs.
	// If FindSuccessors is nil, content.Successors will be used.
	FindSuccessors func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) ([]ocispec.Descriptor, error)
	// MaxMetadataBytes is the maximum amount of bytes of metadata (images / indexes) to cache.
	MaxMetadataBytes int64
}

func copyNodeData(ctx context.Context, src content.ReadOnlyStorage, dst content.Storage, desc ocispec.Descriptor) error {
	exists, err := dst.Exists(ctx, desc)
	if err != nil || exists {
		return err
	}

	rc, err := src.Fetch(ctx, desc)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	if err := dst.Push(ctx, desc, rc); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return err
	}
	return nil
}

type token struct{}

func copyNodes(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	src content.ReadOnlyStorage,
	dst content.Storage,
	descs []ocispec.Descriptor,
	recurse bool,
	sem chan token,
	opts CopyGraphOptions,
) error {
	return copyDispatch(ctx, cancel, src, dst, uniformNodes(descs, recurse), sem, opts)
}

func copyNode(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	src content.ReadOnlyStorage,
	dst content.Storage,
	desc ocispec.Descriptor,
	recurse bool,
	sem chan token,
	opts CopyGraphOptions,
) error {
	if recurse {
		children, err := opts.FindSuccessors(ctx, src, desc)
		if err != nil {
			<-sem
			return err
		}

		if len(children) > 0 {
			<-sem

			if err := copyNodes(ctx, cancel, src, dst, children, recurse, sem, opts); err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case sem <- token{}:
			}
		}
	}

	defer func() { <-sem }()
	return copyNodeData(ctx, src, dst, desc)
}

func copyPath(ctx context.Context, src content.ReadOnlyStorage, dst content.Storage, path Path, opts CopyGraphOptions) error {
	if opts.FindSuccessors == nil {
		opts.FindSuccessors = content.Successors
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultConcurrency
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	return copyDispatch(ctx, cancel, src, dst, pathNodes(path), make(chan token, opts.Concurrency), opts)
}

// copyDispatch dispatches copy jobs for the given nodes concurrently, bounded
// by sem. The first node error both cancels the remaining work and is returned;
// an early context cancellation without a node error yields ctx.Err().
func copyDispatch(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	src content.ReadOnlyStorage,
	dst content.Storage,
	nodes iter.Seq2[ocispec.Descriptor, bool], // (descriptor, recurse)
	sem chan token,
	opts CopyGraphOptions,
) error {
	var (
		wg      sync.WaitGroup
		errOnce sync.Once
		retErr  error
	)
Dispatch:
	for desc, recurse := range nodes {
		select {
		case <-ctx.Done():
			errOnce.Do(func() { retErr = ctx.Err() })
			break Dispatch
		case sem <- token{}:
			wg.Go(func() {
				if err := copyNode(ctx, cancel, src, dst, desc, recurse, sem, opts); err != nil {
					// record-then-cancel inside one Do: the real error must
					// win over the ctx.Done arm above.
					errOnce.Do(func() {
						retErr = err
						cancel(err)
					})
				}
			})
		}
	}
	wg.Wait()
	return retErr
}

// uniformNodes yields every descriptor with the same recurse flag, in order.
func uniformNodes(descs []ocispec.Descriptor, recurse bool) iter.Seq2[ocispec.Descriptor, bool] {
	return func(yield func(ocispec.Descriptor, bool) bool) {
		for _, d := range descs {
			if !yield(d, recurse) {
				return
			}
		}
	}
}

// pathNodes yields the path bottom-up: leaf first (with recurse), then ancestors.
func pathNodes(path Path) iter.Seq2[ocispec.Descriptor, bool] {
	return func(yield func(ocispec.Descriptor, bool) bool) {
		for i := len(path) - 1; i >= 0; i-- {
			if !yield(path[i], i == len(path)-1) {
				return
			}
		}
	}
}

// CopyGraph copies a path from a source to a destination storage while respecting the CopyGraphOptions.
// A path is copied bottom up, meaning that first the image at the bottom including its successors are copied and
// only then the remaining path bottom-up is copied.
func CopyGraph(ctx context.Context, src content.ReadOnlyStorage, dst content.Storage, path Path, opts CopyGraphOptions) error {
	if opts.MaxMetadataBytes <= 0 {
		opts.MaxMetadataBytes = defaultMaxMetadataBytes
	}

	src = newMetadataProxy(src, opts.MaxMetadataBytes)
	return copyPath(ctx, src, dst, path, opts)
}
