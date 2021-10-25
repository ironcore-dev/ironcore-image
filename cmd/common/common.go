package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/remotes"
	onmetalimage "github.com/onmetal/onmetal-image"

	"github.com/onmetal/onmetal-image/docker"

	"github.com/distribution/distribution/reference"

	"github.com/onmetal/onmetal-image/oci/descriptorutil/matcher"
	"github.com/onmetal/onmetal-image/oci/remote"

	"github.com/onmetal/onmetal-image/oci/store"
)

const (
	RecommendedStorePathFlagName         = "store-path"
	RecommendedDockerConfigPathsFlagName = "docker-config-path"
)

const (
	RecommendedStorePathFlagUsage         = "Path where to store all local images and index information (such as tags)."
	RecommendedDockerConfigPathsFlagUsage = "Paths to look up for docker configuration. Leave empty for default location."
)

var (
	// DefaultStorePath is the default store path. If your user does not have a home directory,
	// this is empty and needs to be passed in as a flag.
	DefaultStorePath string
)

func init() {
	if homeDir, err := os.UserHomeDir(); err == nil {
		DefaultStorePath = filepath.Join(homeDir, ".onmetal")
	}
}

// StoreFactory is a factory for a store.Store.
type StoreFactory func() (*store.Store, error)

// DefaultStoreFactory returns a new StoreFactory that dereferences the store path at invocation time.
func DefaultStoreFactory(storePath *string) StoreFactory {
	return func() (*store.Store, error) {
		return store.New(*storePath)
	}
}

// RemoteRegistryFactory is a factory for a remote.Registry.
type RemoteRegistryFactory func() (*remote.Registry, error)

func DefaultRemoteRegistryFactory(configPaths []string) RemoteRegistryFactory {
	return func() (*remote.Registry, error) {
		return remote.DockerRegistry(configPaths)
	}
}

type RequestResolverFactory func() (*docker.RequestResolver, error)

func DefaultRequestResolverFactory(configPaths []string) RequestResolverFactory {
	return func() (*docker.RequestResolver, error) {
		return docker.NewRequestResolver(docker.RequestResolverOptions{
			ConfigPaths: configPaths,
		})
	}
}

func FuzzyResolveRef(ctx context.Context, store *store.Store, ref string) (string, error) {
	if _, err := reference.ParseAnyReference(ref); err == nil {
		return ref, nil
	}

	dsc, err := store.Layout().Indexer().Find(ctx, matcher.EncodedDigestPrefix(ref))
	if err != nil {
		return "", fmt.Errorf("error looking up ref %s as digest: %w", ref, err)
	}

	return dsc.Digest.String(), nil
}

// SetupContext sets up context.Context to not log warnings on onmetal media types.
func SetupContext(ctx context.Context) context.Context {
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, onmetalimage.ConfigMediaType, "config-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, onmetalimage.RootFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, onmetalimage.InitRAMFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, onmetalimage.KernelLayerMediaType, "layer-")
	return ctx
}
