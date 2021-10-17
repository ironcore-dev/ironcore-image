// Copyright 2021 OnMetal authors
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

package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onmetal/onmetal-image/refutil"

	"github.com/containerd/containerd/content/local"

	"github.com/onmetal/onmetal-image/indexer"

	"github.com/distribution/distribution/reference"

	"github.com/containerd/containerd/images"

	"github.com/opencontainers/image-spec/specs-go"

	"github.com/onmetal/onmetal-image/contentutil"

	"oras.land/oras-go/pkg/auth"

	"github.com/containerd/containerd/remotes/docker"

	image "github.com/onmetal/onmetal-image"

	containerdcontent "github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
)

type Client struct {
	authorizer auth.Client
	store      containerdcontent.Store
	indexer    indexer.Indexer
}

func (c *Client) buildImage(ctx context.Context, rootFSPath, initRAMFSPath, vmlinuzPath string) (ocispec.Descriptor, error) {
	rootDesc, err := contentutil.WriteFileToIngester(ctx, c.store, rootFSPath, contentutil.WithMediaType(image.RootFSLayerMediaType))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not write rootfs to ingester: %w", err)
	}

	initRAMFSDesc, err := contentutil.WriteFileToIngester(ctx, c.store, initRAMFSPath, contentutil.WithMediaType(image.InitRAMFSLayerMediaType))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not write initramfs to ingester: %w", err)
	}

	vmlinuzDesc, err := contentutil.WriteFileToIngester(ctx, c.store, vmlinuzPath, contentutil.WithMediaType(image.VMLinuzLayerMediaType))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not write vmlinuz to ingester: %w", err)
	}

	cfg := &image.Image{}
	cfgDesc, err := contentutil.WriteJSONEncodedValueToIngester(ctx, c.store, cfg, contentutil.WithMediaType(image.ConfigMediaType))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not write image config to ingester: %w", err)
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: cfgDesc,
		Layers: []ocispec.Descriptor{
			rootDesc, initRAMFSDesc, vmlinuzDesc,
		},
	}

	desc, err := contentutil.WriteJSONEncodedValueToIngester(ctx, c.store, manifest, contentutil.WithMediaType(ocispec.MediaTypeImageManifest))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not write manifest to ingester: %w", err)
	}

	return desc, nil
}

type BuildOptions struct {
	Reference reference.Named
}

func (o *BuildOptions) ApplyToBuildOptions(o2 *BuildOptions) {
	if o.Reference != nil {
		o2.Reference = o.Reference
	}
}

func (o *BuildOptions) ApplyOptions(opts []BuildOption) {
	for _, opt := range opts {
		opt.ApplyToBuildOptions(o)
	}
}

type BuildOption interface {
	ApplyToBuildOptions(o *BuildOptions)
}

func (c *Client) Build(ctx context.Context, rootFSPath, initRAMFSPath, vmlinuzPath string, opts ...BuildOption) (ocispec.Descriptor, error) {
	ctx = setupMediaTypeContext(ctx)
	o := &BuildOptions{}
	o.ApplyOptions(opts)

	desc, err := c.buildImage(ctx, rootFSPath, initRAMFSPath, vmlinuzPath)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error building image: %w", err)
	}

	if err := c.indexer.Index(ctx, desc); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not index descriptor: %w", err)
	}

	if o.Reference != nil {
		if _, err := c.Tag(ctx, desc, o.Reference); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("could not tag descriptor: %w", err)
		}
	}

	return desc, nil
}

func (c *Client) Push(ctx context.Context, ref reference.Named) (ocispec.Descriptor, error) {
	ctx = setupMediaTypeContext(ctx)

	desc, err := c.indexer.GetByReference(ctx, ref, indexer.WithMediaType(ocispec.MediaTypeImageManifest))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unknown reference %s: %w", ref, err)
	}

	resolver, err := c.resolver()
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not obtain resolver: %w", err)
	}

	pusher, err := resolver.Pusher(ctx, ref.String())
	if err != nil {
		return ocispec.Descriptor{}, nil
	}

	if err := remotes.PushContent(ctx, pusher, desc, c.store, nil, nil, nil); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error pushing to %s: %w", ref, err)
	}

	return desc, nil
}

func (c *Client) resolver() (remotes.Resolver, error) {
	if c.authorizer != nil {
		authClient, err := dockerauth.NewClient()
		if err != nil {
			return nil, fmt.Errorf("could not create auth client: %w", err)
		}

		return authClient.ResolverWithOpts()
	}

	return docker.NewResolver(docker.ResolverOptions{}), nil
}

// setupMediaTypeContext is set up to disable nasty warnings due to unknown layer types.
func setupMediaTypeContext(ctx context.Context) context.Context {
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, image.ConfigMediaType, "config-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, image.RootFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, image.InitRAMFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, image.VMLinuzLayerMediaType, "layer-")
	return ctx
}

func (c *Client) Pull(ctx context.Context, ref reference.Named) (ocispec.Descriptor, error) {
	ctx = setupMediaTypeContext(ctx)

	resolver, err := c.resolver()
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	_, desc, err := resolver.Resolve(ctx, ref.String())
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error resolving %s: %w", ref, err)
	}

	fetcher, err := resolver.Fetcher(ctx, ref.String())
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error getting fetcher for ref %s: %w", ref, err)
	}

	if err := images.Dispatch(ctx, images.Handlers(
		remotes.FetchHandler(c.store, fetcher),
		images.ChildrenHandler(c.store),
	), nil, desc); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error pulling: %w", err)
	}

	if err := c.indexer.Index(ctx, desc); err != nil {
		return ocispec.Descriptor{}, err
	}

	if _, err := c.Tag(ctx, desc, ref); err != nil {
		return ocispec.Descriptor{}, err
	}

	return desc, nil
}

func (c *Client) Tag(ctx context.Context, desc ocispec.Descriptor, target reference.Named) (ocispec.Descriptor, error) {
	refDesc, err := refutil.DescriptorWithReference(desc, target)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := c.indexer.Index(ctx, refDesc); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error indexing ref descriptor: %w", err)
	}
	return desc, nil
}

func (c *Client) Indexer() indexer.Indexer {
	return c.indexer
}

type Options struct {
	ImageBasePath string
	Authorizer    auth.Client
	NewStore      func(string) (containerdcontent.Store, error)
	NewIndexer    func(string) (indexer.Indexer, error)
}

func NewOptions() *Options {
	return &Options{
		NewStore:   local.NewStore,
		NewIndexer: indexer.New,
	}
}

func (o *Options) ApplyOptions(opts []Option) {
	for _, opt := range opts {
		opt.ApplyToOptions(o)
	}
}

type Option interface {
	ApplyToOptions(opts *Options)
}

type withAuthorizer struct {
	client auth.Client
}

func WithAuthorizer(c auth.Client) Option {
	return &withAuthorizer{c}
}

func (w *withAuthorizer) ApplyToOptions(opts *Options) {
	opts.Authorizer = w.client
}

func New(opts ...Option) (*Client, error) {
	o := NewOptions()
	o.ApplyOptions(opts)

	if o.ImageBasePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("no image base path set and could not determine user home director: %w", err)
		}

		o.ImageBasePath = filepath.Join(homeDir, ".onmetal")
	}

	store, err := o.NewStore(o.ImageBasePath)
	if err != nil {
		return nil, fmt.Errorf("error creating store: %w", err)
	}

	index, err := o.NewIndexer(o.ImageBasePath)
	if err != nil {
		return nil, fmt.Errorf("error creating indexer: %w", err)
	}

	return &Client{
		authorizer: o.Authorizer,
		indexer:    index,
		store:      store,
	}, nil
}
