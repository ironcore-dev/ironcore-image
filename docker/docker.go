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

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/containerd/containerd/remotes/docker"

	"github.com/containerd/containerd/remotes"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/distribution/distribution/reference"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
)

type RequestResolver struct {
	resolver remotes.Resolver
	hosts    docker.RegistryHosts
}

type Info interface {
	Request() Request
}

type Request struct {
	Headers http.Header `json:"headers,omitempty"`
	URL     URL         `json:"url"`
}

type URL url.URL

func (u URL) MarshalText() ([]byte, error) {
	tmp := (url.URL)(u)
	return []byte(tmp.String()), nil
}

func (u *URL) UnmarshalText(p []byte) error {
	tmp, err := url.Parse(string(p))
	if err != nil {
		return err
	}
	*u = *(*URL)(tmp)
	return nil
}

func (u *URL) MarshalJSON() ([]byte, error) {
	return u.MarshalText()
}

func (u *URL) UnmarshalJSON(p []byte) error {
	var s string
	if err := json.Unmarshal(p, &s); err != nil {
		return err
	}
	return u.UnmarshalText([]byte(s))
}

type LayerInfo interface {
	Info
	Descriptor() ocispec.Descriptor
}

type ManifestInfo interface {
	Info
	Manifest(ctx context.Context) (*ocispec.Manifest, error)
	Layer(ctx context.Context, desc ocispec.Descriptor) (LayerInfo, error)
}

type baseInfo struct {
	name     string
	client   *http.Client
	registry docker.RegistryHost
	request  Request
}

func (i *baseInfo) baseInit(ctx context.Context, header http.Header, pathAfterName string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fmt.Sprintf("%s://%s%s/%s%s", i.registry.Scheme, i.registry.Host, i.registry.Path, i.name, pathAfterName), nil)
	if err != nil {
		return err
	}

	req.Header = header
	var (
		auth      = i.registry.Authorizer
		responses []*http.Response
	)
	for len(responses) < 6 {
		if err := auth.Authorize(ctx, req); err != nil {
			return fmt.Errorf("error authorizing request: %w", err)
		}

		res, err := i.client.Do(req)
		if err != nil {
			return err
		}
		// Close the body since we won't actually use the response
		_ = res.Body.Close()

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			break
		}

		responses = append(responses, res)
		switch res.StatusCode {
		case http.StatusUnauthorized:
			if err := auth.AddResponses(ctx, responses); err != nil {
				return fmt.Errorf("error adding responses: %w", err)
			}
		// Gracefully handle registries that don't implement HEAD
		case http.StatusMethodNotAllowed:
			req.Method = http.MethodGet
		case http.StatusRequestTimeout, http.StatusTooManyRequests:
		default:
			return fmt.Errorf("erroneous response status: %s for url %s", res.Status, req.URL)
		}
	}

	i.request = Request{
		Headers: req.Header,
		URL:     URL(*req.URL),
	}
	return nil
}

func (i *baseInfo) Request() Request {
	return i.request
}

type manifestInfo struct {
	sync.Once

	ref      string
	tag      string
	resolver remotes.Resolver
	baseInfo

	manifest *ocispec.Manifest
	err      error
}

func (i *manifestInfo) Manifest(ctx context.Context) (*ocispec.Manifest, error) {
	i.Do(func() {
		_, desc, err := i.resolver.Resolve(ctx, i.ref)
		if err != nil {
			i.err = fmt.Errorf("error resolving ref %s: %w", i.ref, err)
			return
		}

		fetcher, err := i.resolver.Fetcher(ctx, i.ref)
		if err != nil {
			i.err = fmt.Errorf("error creating fetcher for ref %s: %w", i.ref, err)
			return
		}

		rc, err := fetcher.Fetch(ctx, desc)
		if err != nil {
			i.err = fmt.Errorf("error opening fetch for ref %s digest %s: %w", i.ref, desc.Digest, err)
			return
		}
		defer func() { _ = rc.Close() }()

		manifest := &ocispec.Manifest{}
		if err := json.NewDecoder(rc).Decode(manifest); err != nil {
			i.err = fmt.Errorf("error decoding manifest: %w", err)
			return
		}

		i.manifest = manifest
	})
	return i.manifest, i.err
}

func (i *manifestInfo) Layer(ctx context.Context, desc ocispec.Descriptor) (LayerInfo, error) {
	manifest, err := i.Manifest(ctx)
	if err != nil {
		return nil, err
	}

	var found bool
	for _, layer := range manifest.Layers {
		if layer.Digest == desc.Digest {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("unknown layer %s", desc.Digest)
	}

	info := &layerInfo{
		desc: desc,
		baseInfo: baseInfo{
			name:     i.name,
			client:   i.client,
			registry: i.registry,
		},
	}
	if err := info.init(ctx); err != nil {
		return nil, fmt.Errorf("error initializing info: %w", err)
	}
	return info, nil
}

func (i *manifestInfo) init(ctx context.Context) error {
	return i.baseInit(ctx, http.Header{
		"Accept": []string{ocispec.MediaTypeImageManifest},
	}, fmt.Sprintf("/manifests/%s", i.tag))
}

type layerInfo struct {
	desc ocispec.Descriptor
	baseInfo
}

func (i *layerInfo) init(ctx context.Context) error {
	return i.baseInit(ctx, http.Header{}, fmt.Sprintf("/blobs/%s", i.desc.Digest))
}

func (i *layerInfo) Descriptor() ocispec.Descriptor {
	return i.desc
}

func (u *RequestResolver) Resolve(ctx context.Context, ref string) (ManifestInfo, error) {
	r, err := reference.ParseNamed(ref)
	if err != nil {
		return nil, fmt.Errorf("ref %s is no named reference: %w", ref, err)
	}

	tag := "latest"
	if tagged, ok := r.(reference.Tagged); ok {
		tag = tagged.Tag()
	}

	host := reference.Domain(r)
	regs, err := u.hosts(host)
	if err != nil {
		return nil, fmt.Errorf("error getting host for %s", host)
	}
	var found *docker.RegistryHost
	for _, reg := range regs {
		if reg.Capabilities.Has(docker.HostCapabilityPull) {
			reg := reg
			found = &reg
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("no registry providing pull capability for host %s", host)
	}

	info := &manifestInfo{
		ref:      ref,
		tag:      tag,
		resolver: u.resolver,
		baseInfo: baseInfo{
			name:     reference.Path(r),
			client:   http.DefaultClient,
			registry: *found,
		},
	}
	if err := info.init(ctx); err != nil {
		return nil, fmt.Errorf("error initializing: %w", err)
	}
	return info, nil
}

type RequestResolverOptions struct {
	ConfigPaths []string
	Client      *http.Client
	Header      http.Header
}

func (o *RequestResolverOptions) SetDefaults() {
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
}

func NewRequestResolver(o RequestResolverOptions) (*RequestResolver, error) {
	o.SetDefaults()

	authC, err := dockerauth.NewClient(o.ConfigPaths...)
	if err != nil {
		return nil, fmt.Errorf("error instantiating client: %w", err)
	}

	c := authC.(*dockerauth.Client)

	resolver, err := c.ResolverWithOpts()
	if err != nil {
		return nil, fmt.Errorf("error instantiating resolver: %w", err)
	}

	authorizer := docker.NewDockerAuthorizer(
		docker.WithAuthClient(o.Client),
		docker.WithAuthCreds(c.Credential),
		docker.WithAuthHeader(o.Header),
	)

	hosts := docker.ConfigureDefaultRegistries(
		docker.WithPlainHTTP(docker.MatchLocalhost),
		docker.WithAuthorizer(authorizer),
	)

	return &RequestResolver{
		resolver: resolver,
		hosts:    hosts,
	}, nil
}
