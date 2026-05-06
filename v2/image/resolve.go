package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"slices"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

type PullPolicy uint8

const (
	PullIfNotPresent PullPolicy = iota
	PullAlways
	PullNever
)

type GetOptions struct {
	// MatchArtifactType only matches image manifests with artifact types that pass this function.
	// Other images are skipped.
	MatchArtifactType func(string) bool
	// MatchPlatform only matches image manifests with platforms that pass this function.
	// If a nested index specifies a platform, it must match this, otherwise it's not visited.
	MatchPlatform func(*ocispec.Platform) bool
	// CompareDescriptors allows comparing multiple matching descriptors and ranking them.
	// If omitted, the first matching descriptor is returned.
	CompareDescriptors func(a, b ocispec.Descriptor) int
	// MaxMetadataBytes is the maximum amount of bytes of metadata (images / indexes) to cache.
	MaxMetadataBytes int64
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

	PullPolicy PullPolicy
}

// IgnoreImageOrIndexNotFound ignores errdef.ErrNotFound if it happened on
// a Path.Base descriptor that points to an image index or an image manifest.
func IgnoreImageOrIndexNotFound(p Path, err error) error {
	if err == nil || !errors.Is(err, errdef.ErrNotFound) || len(p) == 0 {
		return err
	}
	if p.Base().MediaType == ocispec.MediaTypeImageIndex || p.Base().MediaType == ocispec.MediaTypeImageManifest {
		return nil
	}
	return err
}

func Get(
	ctx context.Context,
	local oras.Target,
	ref string,
	mkRemote func(ctx context.Context, ref string) (oras.ReadOnlyTarget, string, error),
	opts GetOptions,
) (ocispec.Descriptor, error) {
	if opts.PullPolicy == PullAlways {
		remote, remoteRef, err := mkRemote(ctx, ref)
		if err != nil {
			return ocispec.Descriptor{}, err
		}

		return Pull(ctx, remote, local, remoteRef, ref, PullOptions{
			MatchArtifactType:  opts.MatchArtifactType,
			MatchPlatform:      opts.MatchPlatform,
			CompareDescriptors: opts.CompareDescriptors,
			MaxMetadataBytes:   opts.MaxMetadataBytes,
			Concurrency:        opts.Concurrency,
			FindSuccessors:     opts.FindSuccessors,
		})
	}

	desc, err := Resolve(ctx, local, ref, ResolveOptions{
		MatchArtifactType:  opts.MatchArtifactType,
		MatchPlatform:      opts.MatchPlatform,
		CompareDescriptors: opts.CompareDescriptors,
		OnIterateError:     IgnoreImageOrIndexNotFound,
	})
	if err == nil {
		ok, err := local.Exists(ctx, desc)
		if err != nil || ok {
			return desc, err
		}
		err = fmt.Errorf("descriptor %s %w", desc.Digest.String(), errdef.ErrNotFound)
	}
	if err != nil && !errors.Is(err, errdef.ErrNotFound) {
		return ocispec.Descriptor{}, fmt.Errorf("locally resolving %q: %w", ref, err)
	}

	if opts.PullPolicy != PullIfNotPresent {
		return ocispec.Descriptor{}, err
	}
	remote, remoteRef, err := mkRemote(ctx, ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return Pull(ctx, remote, local, remoteRef, ref, PullOptions{
		MatchArtifactType:  opts.MatchArtifactType,
		MatchPlatform:      opts.MatchPlatform,
		CompareDescriptors: opts.CompareDescriptors,
		MaxMetadataBytes:   opts.MaxMetadataBytes,
		Concurrency:        opts.Concurrency,
		FindSuccessors:     opts.FindSuccessors,
	})
}

// PullOptions are options for the Pull function.
type PullOptions struct {
	// MatchArtifactType only matches image manifests with artifact types that pass this function.
	// Other images are skipped.
	MatchArtifactType func(string) bool
	// MatchPlatform only matches image manifests with platforms that pass this function.
	// If a nested index specifies a platform, it must match this, otherwise it's not visited.
	MatchPlatform func(*ocispec.Platform) bool
	// CompareDescriptors allows comparing multiple matching descriptors and ranking them.
	// If omitted, the first matching descriptor is returned.
	CompareDescriptors func(a, b ocispec.Descriptor) int
	// OnIterateError specifies what to do when encountering an error during image iteration.
	OnIterateError func(p Path, err error) error
	// MaxMetadataBytes is the maximum amount of bytes of metadata (images / indexes) to cache.
	MaxMetadataBytes int64
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
}

func Pull(ctx context.Context, src oras.ReadOnlyTarget, dst oras.Target, srcRef, dstRef string, opts PullOptions) (ocispec.Descriptor, error) {
	root, err := src.Resolve(ctx, srcRef)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	proxy := newMetadataProxy(src, opts.MaxMetadataBytes)
	p, err := resolve(ctx, src, root, ResolveOptions{
		MatchArtifactType:  opts.MatchArtifactType,
		MatchPlatform:      opts.MatchPlatform,
		CompareDescriptors: opts.CompareDescriptors,
		OnIterateError:     opts.OnIterateError,
	})
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := copyPath(ctx, proxy, dst, p, CopyGraphOptions{
		Concurrency:    opts.Concurrency,
		FindSuccessors: opts.FindSuccessors,
	}); err != nil {
		return ocispec.Descriptor{}, err
	}

	return p.Base(), dst.Tag(ctx, root, dstRef)
}

type ResolveOptions struct {
	// MatchArtifactType only matches image manifests with artifact types that pass this function.
	// Other images are skipped.
	MatchArtifactType func(string) bool
	// MatchPlatform only matches image manifests with platforms that pass this function.
	// If a nested index specifies a platform, it must match this, otherwise it's not visited.
	MatchPlatform func(*ocispec.Platform) bool
	// CompareDescriptors allows comparing multiple matching descriptors and ranking them.
	// If omitted, the first matching descriptor is returned.
	CompareDescriptors func(a, b ocispec.Descriptor) int
	// OnIterateError specifies what to do when encountering an error during image iteration.
	OnIterateError func(p Path, err error) error
}

func Resolve(ctx context.Context, src oras.ReadOnlyTarget, ref string, opts ResolveOptions) (ocispec.Descriptor, error) {
	root, err := src.Resolve(ctx, ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	p, err := resolve(ctx, src, root, opts)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return p.Base(), nil
}

func resolve(ctx context.Context, src content.ReadOnlyStorage, root ocispec.Descriptor, opts ResolveOptions) (Path, error) {
	imagesOpts := ImagesOptions{
		MatchArtifactType: opts.MatchArtifactType,
		MatchPlatform:     opts.MatchPlatform,
	}

	var matching []Path
	for p, err := range Images(ctx, src, root, imagesOpts) {
		if err != nil {
			if opts.OnIterateError != nil {
				if err := opts.OnIterateError(p, err); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}

		if opts.CompareDescriptors == nil {
			return p, nil
		}
		matching = append(matching, p)
	}
	if len(matching) == 0 {
		return nil, errdef.ErrNotFound
	}
	slices.SortStableFunc(matching, func(a, b Path) int {
		return opts.CompareDescriptors(a.Base(), b.Base())
	})
	return matching[0], nil
}

// ImagesOptions are options for the Iterate function.
type ImagesOptions struct {
	// MatchArtifactType only matches image manifests with artifact types that pass this function.
	// Other images are skipped.
	MatchArtifactType func(string) bool
	// MatchPlatform only matches image manifests with platforms that pass this function.
	// If a nested index specifies a platform, it must match this, otherwise it's not visited.
	MatchPlatform func(*ocispec.Platform) bool
}

// Images iterates all image manifests by walking the graph rooted at the given descriptor respecting
// the given IterateOptions.
func Images(
	ctx context.Context,
	src content.ReadOnlyStorage,
	root ocispec.Descriptor,
	opts ImagesOptions,
) iter.Seq2[Path, error] {
	return func(yield func(path Path, err error) bool) {
		canYield := true
		err := Walk(ctx, src, root, func(path Path, err error) error {
			if err != nil {
				if !yield(path, err) {
					canYield = false
					return ErrSkipAll
				}
				return nil
			}

			desc := path.Base()
			switch desc.MediaType {
			case ocispec.MediaTypeImageIndex:
				if opts.MatchPlatform != nil && desc.Platform != nil && !opts.MatchPlatform(desc.Platform) {
					return ErrSkipNode
				}
				return nil
			case ocispec.MediaTypeImageManifest:
				if opts.MatchArtifactType != nil && !opts.MatchArtifactType(desc.ArtifactType) {
					return nil
				}
				if opts.MatchPlatform != nil && !opts.MatchPlatform(desc.Platform) {
					return nil
				}
				if !yield(path, nil) {
					canYield = false
					return ErrSkipAll
				}
				return nil
			default:
				// Unreachable by walk's contract: fn is called with a nil error only
				// for image indexes and manifests (unsupported types arrive via the
				// error path above). Kept as a safety net in case walk ever delivers
				// other node types successfully.
				err := fmt.Errorf("%w: not an index or image (%s)", errdef.ErrUnsupported, desc.MediaType)
				if !yield(path, err) {
					canYield = false
					return ErrSkipAll
				}
				return nil
			}
		})
		if err != nil && canYield {
			yield(nil, err)
		}
	}
}

type Path []ocispec.Descriptor

func (p Path) Base() ocispec.Descriptor {
	return p[len(p)-1]
}

var (
	// ErrSkipNode can be returned from WalkFunc to skip the current node only.
	ErrSkipNode = errors.New("skip node")
	// ErrSkipAll can be returned from WalkFunc to exit the walk early and skip all remaining nodes.
	ErrSkipAll = errors.New("skip all")
)

func stat(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex:
		data, err := content.FetchAll(ctx, fetcher, desc)
		if err != nil {
			return ocispec.Descriptor{}, err
		}

		index := &ocispec.Index{}
		if err := json.Unmarshal(data, index); err != nil {
			return ocispec.Descriptor{}, err
		}

		desc.ArtifactType = index.ArtifactType
		desc.Annotations = index.Annotations
		return desc, nil
	case ocispec.MediaTypeImageManifest:
		data, err := content.FetchAll(ctx, fetcher, desc)
		if err != nil {
			return ocispec.Descriptor{}, err
		}

		manifest := &ocispec.Manifest{}
		if err := json.Unmarshal(data, manifest); err != nil {
			return ocispec.Descriptor{}, err
		}

		cfg, _, err := DecodeConfig(ctx, fetcher, desc)
		if err != nil {
			return ocispec.Descriptor{}, err
		}

		desc.ArtifactType = manifest.ArtifactType
		desc.Annotations = manifest.Annotations
		desc.Platform = cfg.Platform
		return desc, nil
	default:
		return desc, fmt.Errorf("%w: not an index or image manifest", errdef.ErrUnsupported)
	}
}

// WalkFunc is a function that is called on each step of Walk.
// A non-nil error indicates there was some error with the base descriptor of the path.
type WalkFunc func(path Path, err error) error

// Walk walks the graph located at the given descriptor.
// Walk traverses the graph in a depth-first manner.
// The given WalkFunc is called with all nodes of the graph.
// If err in the WalkFunc is nil, it is guaranteed that the node exists and is well-formed.
func Walk(
	ctx context.Context,
	src content.ReadOnlyStorage,
	root ocispec.Descriptor,
	fn WalkFunc,
) error {
	root, err := stat(ctx, src, root)
	if err != nil {
		err = fn(Path{root}, err)
	} else {
		err = walk(ctx, src, fn, Path{root})
	}
	if errors.Is(err, ErrSkipAll) || errors.Is(err, ErrSkipNode) {
		return nil
	}
	return err
}

// joinPath returns a new Path with the descriptor appended.
func joinPath(path Path, desc ocispec.Descriptor) Path {
	res := make(Path, len(path)+1)
	copy(res, path)
	res[len(res)-1] = desc
	return res
}

func walk(
	ctx context.Context,
	src content.ReadOnlyStorage,
	fn WalkFunc,
	p Path,
) error {
	if p.Base().MediaType != ocispec.MediaTypeImageIndex {
		return fn(p, nil)
	}

	data, err := content.FetchAll(ctx, src, p.Base())
	if err != nil {
		return fn(p, err)
	}

	index := ocispec.Index{}
	if err := json.Unmarshal(data, &index); err != nil {
		return fn(p, err)
	}

	if err := fn(p, nil); err != nil {
		return err
	}

	for _, child := range index.Manifests {
		if child.MediaType != ocispec.MediaTypeImageIndex && child.MediaType != ocispec.MediaTypeImageManifest {
			continue
		}

		if err := walk(ctx, src, fn, joinPath(p, child)); err != nil && !errors.Is(err, ErrSkipNode) {
			return err
		}
	}
	return nil
}
