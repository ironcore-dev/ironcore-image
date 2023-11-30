// Copyright 2021 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package remote

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	ociimage "github.com/ironcore-dev/ironcore-image/oci/image"
	"oras.land/oras-go/pkg/auth"
	"oras.land/oras-go/pkg/auth/docker"
)

type Registry struct {
	resolver remotes.Resolver
}

func (r *Registry) Resolve(ctx context.Context, ref string) (ociimage.Image, error) {
	_, desc, err := r.resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("error resolving %s: %w", ref, err)
	}

	fetcher, err := r.resolver.Fetcher(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("error getting fetcher for %s: %w", ref, err)
	}

	return Image(fetcher, desc), nil
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
