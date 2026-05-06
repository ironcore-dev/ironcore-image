package image

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

type BuildConfigTypeMeta struct {
	Kind string `json:"kind"`
}

func (t *BuildConfigTypeMeta) GetKind() string {
	return t.Kind
}

func (t *BuildConfigTypeMeta) SetKind(kind string) {
	t.Kind = kind
}

type BuildConfig interface {
	GetKind() string
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

type BuildOptions struct {
	// ExplicitKind explicitly sets the kind of the config file.
	ExplicitKind string
	Platform     *ocispec.Platform
}

type BuilderOptions struct {
	Platform *ocispec.Platform
}

func Expand(s string, opts BuilderOptions) string {
	return os.Expand(s, func(key string) string {
		switch key {
		case "TARGETOS":
			if opts.Platform == nil {
				return ""
			}
			return opts.Platform.OS
		case "TARGETARCH":
			if opts.Platform == nil {
				return ""
			}
			return opts.Platform.Architecture
		case "TARGETVARIANT":
			if opts.Platform == nil {
				return ""
			}
			return opts.Platform.Variant
		default:
			return ""
		}
	})
}

var (
	providersMu     sync.Mutex
	atomicProviders atomic.Value
)

type builder struct {
	kind      string
	newConfig func() BuildConfig
	build     func(ctx context.Context, pusher content.Pusher, buildCtx BuildContext, cfg BuildConfig, opts BuilderOptions) (ocispec.Descriptor, error)
}

func RegisterBuilder(
	kind string,
	newConfig func() BuildConfig,
	build func(ctx context.Context, pusher content.Pusher, buildCtx BuildContext, cfg BuildConfig, opts BuilderOptions) (ocispec.Descriptor, error),
) {
	providersMu.Lock()
	defer providersMu.Unlock()

	providers, _ := atomicProviders.Load().([]builder)
	atomicProviders.Store(append(providers, builder{
		kind:      kind,
		newConfig: newConfig,
		build:     build,
	}))
}

func KnownKinds() []string {
	providersMu.Lock()
	defer providersMu.Unlock()

	providers, _ := atomicProviders.Load().([]builder)
	result := make([]string, 0, len(providers))
	for _, b := range providers {
		result = append(result, b.kind)
	}

	slices.Sort(result)
	return result
}

func findProviderByKind(kind string) (*builder, bool) {
	providers, _ := atomicProviders.Load().([]builder)
	for _, b := range providers {
		if b.kind == kind {
			return &b, true
		}
	}
	return nil, false
}

// ReadConfig reads the data into a config object. Optionally specify kind to indicate
// which config kind to use. Otherwise, kind will be detected dynamically if possible.
func ReadConfig(r io.Reader, explicitKind string) (BuildConfig, string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}

	if explicitKind == "" {
		detectedKind, err := DetectConfigKind(data)
		if err != nil {
			return nil, "", err
		}

		explicitKind = detectedKind
	}

	b, ok := findProviderByKind(explicitKind)
	if !ok {
		return nil, "", fmt.Errorf("unknown provider kind %q", explicitKind)
	}

	cfg := b.newConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, b.kind, fmt.Errorf("unmarshalling config kind %q: %w", explicitKind, err)
	}

	return cfg, b.kind, nil
}

func KindDefaultConfigName(kind string) string {
	return fmt.Sprintf("%sfile", kind)
}

func DetectConfigKindFromFilename(filename string) string {
	for _, knownKind := range KnownKinds() {
		if KindDefaultConfigName(knownKind) == filepath.Base(filename) {
			return knownKind
		}
	}
	return ""
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

func DetectConfigKindFromFile(filename string) (string, error) {
	if kind := DetectConfigKindFromFilename(filename); kind != "" {
		return kind, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return DetectConfigKind(data)
}

// ReadConfigFile reads the config file at the specified path into a config object. Optionally specify an explicit kind
// to indicate which config kind to use. Otherwise, kind will be detected dynamically if possible.
func ReadConfigFile(filename, explicitKind string) (BuildConfig, string, error) {
	rc, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rc.Close() }()

	if explicitKind == "" {
		explicitKind = DetectConfigKindFromFilename(filename)
	}

	return ReadConfig(rc, explicitKind)
}

func Build(ctx context.Context, pusher content.Pusher, bCtx BuildContext, r io.Reader, opts BuildOptions) (ocispec.Descriptor, string, error) {
	cfg, kind, err := ReadConfig(r, opts.ExplicitKind)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}

	b, ok := findProviderByKind(kind)
	if !ok {
		return ocispec.Descriptor{}, "", fmt.Errorf("%w: unknown kind %q", errdef.ErrUnsupported, cfg.GetKind())
	}

	desc, err := b.build(ctx, pusher, bCtx, cfg, BuilderOptions{
		Platform: opts.Platform,
	})
	return desc, b.kind, err
}

type BuildFileOptions struct {
	// ExplicitKind explicitly sets the kind of the config file.
	ExplicitKind string

	Platform *ocispec.Platform
}

func BuildFile(ctx context.Context, pusher content.Pusher, bCtx BuildContext, filename string, opts BuildFileOptions) (ocispec.Descriptor, string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}
	defer func() { _ = f.Close() }()

	if opts.ExplicitKind == "" {
		opts.ExplicitKind = DetectConfigKindFromFilename(filename)
	}

	return Build(ctx, pusher, bCtx, f, BuildOptions{
		ExplicitKind: opts.ExplicitKind,
		Platform:     opts.Platform,
	})
}
