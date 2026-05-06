// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ironcore-dev/ironcore-image/v2/xoci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2/content"
)

type TypeMeta struct {
	Kind string `json:"kind"`
}

func (t *TypeMeta) GetKind() string {
	return t.Kind
}

func (t *TypeMeta) SetKind(kind string) {
	t.Kind = kind
}

type Config interface {
	GetKind() string
	SetKind(kind string)
}

func ParsePlatform(platform string) (*ocispec.Platform, error) {
	parts := strings.SplitN(platform, "/", 4)
	if len(parts) == 4 {
		return nil, fmt.Errorf("invalid platform %q", platform)
	}

	p := &ocispec.Platform{OS: parts[0]}
	if len(parts) > 1 {
		p.Architecture = parts[1]
	}
	if len(parts) > 2 {
		p.Variant = parts[2]
	}
	return p, nil
}

type BuildOptions struct {
	OS           string
	Architecture string
	Variant      string
}

func Expand(s string, opts BuildOptions) string {
	return os.Expand(s, func(key string) string {
		switch key {
		case "TARGETOS":
			return opts.OS
		case "TARGETARCH":
			return opts.Architecture
		case "TARGETVARIANT":
			return opts.Variant
		default:
			return ""
		}
	})
}

type Provider interface {
	NewConfig() Config
	Build(ctx context.Context, pusher content.Pusher, buildCtx BuildContext, cfg Config, opts BuildOptions) (ocispec.Descriptor, error)
	Inspect(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Image, error)
}

var (
	providersMu     sync.Mutex
	atomicProviders atomic.Value
)

type provider struct {
	kind         string
	artifactType string
	provider     Provider
}

func RegisterProvider(kind, artifactType string, prov Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()

	providers, _ := atomicProviders.Load().([]provider)
	atomicProviders.Store(append(providers, provider{
		kind:         kind,
		artifactType: artifactType,
		provider:     prov,
	}))
}

func KnownKinds() []string {
	providersMu.Lock()
	defer providersMu.Unlock()

	providers, _ := atomicProviders.Load().([]provider)
	result := make([]string, 0, len(providers))
	for _, b := range providers {
		result = append(result, b.kind)
	}

	slices.Sort(result)
	return result
}

func findProviderByKind(kind string) (*provider, bool) {
	providers, _ := atomicProviders.Load().([]provider)
	for _, b := range providers {
		if b.kind == kind {
			return &b, true
		}
	}
	return nil, false
}

func findProviderByArtifactType(artifactType string) (*provider, bool) {
	providers, _ := atomicProviders.Load().([]provider)
	for _, b := range providers {
		if b.artifactType == artifactType {
			return &b, true
		}
	}
	return nil, false
}

// ReadConfig reads the data into a config object. Optionally specify kind to indicate
// which config kind to use. Otherwise, kind will be detected dynamically if possible.
func ReadConfig(data []byte, kind string) (Config, error) {
	if kind == "" {
		detectedKind, err := DetectConfigKind(data)
		if err != nil {
			return nil, err
		}

		kind = detectedKind
	}

	b, ok := findProviderByKind(kind)
	if !ok {
		return nil, fmt.Errorf("unknown provider kind %q", kind)
	}

	cfg := b.provider.NewConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config kind %q: %w", kind, err)
	}

	cfg.SetKind(kind)
	return cfg, nil
}

func KindDefaultConfigName(kind string) string {
	return fmt.Sprintf("%sfile", kind)
}

func DetectConfigKind(data []byte) (string, error) {
	detectCfg := struct {
		Kind string `json:"kind"`
	}{}
	if err := yaml.Unmarshal(data, &detectCfg); err != nil {
		return "", fmt.Errorf("unmarshalling config: %w", err)
	}
	if detectCfg.Kind == "" {
		return "", fmt.Errorf("no kind found in config")
	}
	return detectCfg.Kind, nil
}

// ReadConfigFile reads the config file at the specified path into a config object. Optionally specify kind to indicate
// which config kind to use. Otherwise, kind will be detected dynamically if possible.
func ReadConfigFile(filename, kind string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", filename, err)
	}

	if kind == "" {
		if potentialKind, ok := strings.CutSuffix(filepath.Base(filename), "file"); ok {
			if slices.Contains(KnownKinds(), potentialKind) {
				kind = potentialKind
			}
		}
	}
	return ReadConfig(data, kind)
}

type BuildContext interface {
	fs.FS
	io.Closer
}

type osRootBuildContext struct {
	root *os.Root
}

func (b *osRootBuildContext) Open(name string) (fs.File, error) {
	return b.root.Open(name)
}

func (b *osRootBuildContext) Close() error {
	return b.root.Close()
}

func OpenOSRootBuildContext(name string) (BuildContext, error) {
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	return &osRootBuildContext{root: root}, nil
}

type Image interface {
	GetManifest() *ocispec.Manifest
}

var (
	ErrUnknownArtifactType = errors.New("unknown artifact type")
	ErrUnknownKind         = errors.New("unknown kind")
	ErrNoMatchingManifest  = errors.New("no matching manifest")
)

func Build(ctx context.Context, pusher content.Pusher, buildCtx BuildContext, cfg Config, opts BuildOptions) (ocispec.Descriptor, error) {
	b, ok := findProviderByKind(cfg.GetKind())
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("%w %q", ErrUnknownKind, cfg.GetKind())
	}

	return b.provider.Build(ctx, pusher, buildCtx, cfg, opts)
}

func Inspect(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) (Image, string, error) {
	b, ok := findProviderByArtifactType(desc.ArtifactType)
	if !ok {
		return nil, "", fmt.Errorf("%w %q", ErrUnknownArtifactType, desc.ArtifactType)
	}

	img, err := b.provider.Inspect(ctx, fetcher, desc)
	if err != nil {
		return nil, "", err
	}

	return img, b.kind, nil
}

type ResolveOptions struct {
	KindPlatforms          map[string]*ocispec.Platform
	KindAndPlatformMatcher Matcher
}

type Matcher interface {
	Match(kind string, platform *ocispec.Platform) bool
	Less(kind1 string, platform1 *ocispec.Platform, kind2 string, platform2 *ocispec.Platform) bool
}

func PlatformMatches(matcher, actual *ocispec.Platform) bool {
	switch {
	case matcher == nil:
		return true
	case actual != nil:
		if matcher.OS != "" && matcher.OS != actual.OS {
			return false
		}
		if matcher.Architecture != "" && matcher.Architecture != actual.Architecture {
			return false
		}
		if matcher.Variant != "" && matcher.Variant != actual.Variant {
			return false
		}
		return true
	default:
		return false
	}
}

func Resolve(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor, opts ResolveOptions) (ocispec.Descriptor, Image, string, error) {
	type match struct {
		desc ocispec.Descriptor
		img  Image
		kind string
	}

	var matches []match
	if err := xoci.Walk(ctx, fetcher, desc, func(ctx context.Context, desc ocispec.Descriptor, err error) error {
		if err != nil {
			return err
		}

		if desc.MediaType != ocispec.MediaTypeImageManifest {
			return nil
		}

		img, kind, err := Inspect(ctx, fetcher, desc)
		if err != nil {
			if !errors.Is(err, ErrUnknownArtifactType) {
				return err
			}
			return nil
		}

		if matcher := opts.KindAndPlatformMatcher; matcher != nil {
			if matcher.Match(kind, desc.Platform) {
				matches = append(matches, match{
					desc: desc,
					img:  img,
					kind: kind,
				})
			}
			return nil
		}

		if len(opts.KindPlatforms) > 0 {
			platformByKind, ok := opts.KindPlatforms[kind]
			if !ok {
				return nil
			}

			if !PlatformMatches(platformByKind, desc.Platform) {
				return nil
			}
			matches = append(matches, match{
				desc: desc,
				img:  img,
				kind: kind,
			})
			return xoci.ErrSkipAll
		}

		matches = append(matches, match{
			desc: desc,
			img:  img,
			kind: kind,
		})
		return xoci.ErrSkipAll
	}); err != nil {
		return ocispec.Descriptor{}, nil, "", err
	}
	switch len(matches) {
	case 0:
		return ocispec.Descriptor{}, nil, "", ErrNoMatchingManifest
	case 1:
		return matches[0].desc, matches[0].img, matches[0].kind, nil
	default:
	}

	slices.SortFunc(matches, func(a, b match) int {
		if opts.KindAndPlatformMatcher.Less(a.kind, a.desc.Platform, b.kind, b.desc.Platform) {
			return -1
		}
		return 1
	})
	return matches[0].desc, matches[0].img, matches[0].kind, nil
}
