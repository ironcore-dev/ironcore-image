// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/errdefs"
	ocicontent "github.com/ironcore-dev/ironcore-image/oci/content"
	ociimage "github.com/ironcore-dev/ironcore-image/oci/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/pkg/auth"
	"oras.land/oras-go/pkg/auth/docker"
)

type Registry struct {
	resolver remotes.Resolver
}

func (r *Registry) Resolve(ctx context.Context, ref string, platform *ocispec.Platform) (ociimage.Image, error) {
	_, desc, err := r.resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("error resolving %s: %w", ref, err)
	}

	fetcher, err := r.resolver.Fetcher(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("error getting fetcher for %s: %w", ref, err)
	}

	switch desc.MediaType {
	case ocispec.MediaTypeImageManifest:
		return Image(fetcher, desc), nil

	case ocispec.MediaTypeImageIndex:
		rc, err := fetcher.Fetch(ctx, desc)
		if err != nil {
			return nil, fmt.Errorf("error fetching index blob: %w", err)
		}
		defer rc.Close()

		var indexManifest ocispec.Index
		if err := json.NewDecoder(rc).Decode(&indexManifest); err != nil {
			return nil, fmt.Errorf("error decoding image index manifest: %w", err)
		}

		matched := matchPlatform(indexManifest.Manifests, platform)
		if matched == nil {
			return nil, fmt.Errorf("no matching platform found in index for platform %+v", platform)
		}

		return Image(fetcher, *matched), nil

	default:
		return nil, fmt.Errorf("unsupported media type: %s", desc.MediaType)
	}
}

func matchPlatform(manifests []ocispec.Descriptor, target *ocispec.Platform) *ocispec.Descriptor {
	if target == nil {
		if len(manifests) > 0 {
			return &manifests[0]
		}
		return nil
	}

	for _, m := range manifests {
		if m.Platform == nil {
			continue
		}
		if m.Platform.OS == target.OS && m.Platform.Architecture == target.Architecture {
			return &m
		}
	}
	return nil
}

func (r *Registry) pushLayer(ctx context.Context, pusher remotes.Pusher, layer ociimage.Layer) error {
	w, err := pusher.Push(ctx, layer.Descriptor())
	if err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return fmt.Errorf("error getting writer: %w", err)
		}
		return nil
	}

	rc, err := layer.Content(ctx)
	if err != nil {
		_ = w.Close()
		return fmt.Errorf("error getting layer content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	if err := content.Copy(ctx, w, rc, layer.Descriptor().Size, layer.Descriptor().Digest); err != nil {
		_ = w.Close()
		return fmt.Errorf("error copying layer: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("error closing writer: %w", err)
	}
	return nil
}

func (r *Registry) Push(ctx context.Context, ref string, img ociimage.Image) error {

	if img.Descriptor().MediaType == ocispec.MediaTypeImageIndex {
		pusher, err := r.resolver.Pusher(ctx, ref)
		if err != nil {
			return fmt.Errorf("error getting pusher for %s: %w", ref, err)
		}

		w, err := pusher.Push(ctx, img.Descriptor())
		if err != nil {
			if !errdefs.IsAlreadyExists(err) {
				return fmt.Errorf("error getting writer for index manifest: %w", err)
			}
			return nil
		}

		indexData, err := ocicontent.GetIndexManifest(ctx, img)
		if err != nil {
			_ = w.Close()
			return fmt.Errorf("error getting index manifest content: %w", err)
		}

		encodedData, err := json.Marshal(indexData)
		if err != nil {
			_ = w.Close()
			return fmt.Errorf("error marshaling index manifest: %w", err)
		}
		if err := content.Copy(ctx, w, bytes.NewReader(encodedData), img.Descriptor().Size, img.Descriptor().Digest); err != nil {
			return fmt.Errorf("error copying index manifest: %w", err)
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("error closing writer for index manifest: %w", err)
		}
		return nil
	}

	pusher, err := r.resolver.Pusher(ctx, ref)
	if err != nil {
		return fmt.Errorf("error getting pusher for %s: %w", ref, err)
	}

	layers, err := ociimage.AsWriteLayers(ctx, img)
	if err != nil {
		return fmt.Errorf("error transforming image to write layers: %w", err)
	}

	for _, layer := range layers {
		if err := r.pushLayer(ctx, pusher, layer); err != nil {
			return fmt.Errorf("error pushing layer %s: %w", layer.Descriptor().Digest, err)
		}
	}
	return nil
}

func DockerRegistry(configPaths []string, opts ...auth.ResolverOption) (*Registry, error) {
	dockerClient, err := docker.NewClient(configPaths...)
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %w", err)
	}

	resolver, err := dockerClient.ResolverWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating resolver: %w", err)
	}

	return &Registry{resolver}, nil
}
