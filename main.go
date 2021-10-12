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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/containerd/containerd/images"

	"github.com/containerd/containerd/remotes"

	containerdcontent "github.com/containerd/containerd/content"

	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sethvargo/go-signalcontext"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
)

const (
	ConfigMediaType         = "application/vnd.onmetal.image.config.v1alpha1+json"
	RootFSLayerMediaType    = "application/vnd.onmetal.image.rootfs.v1alpha1.rootfs"
	InitRAMFSLayerMediaType = "application/vnd.onmetal.image.initramfs.v1alpha1.initramfs"
	VMLinuzLayerMediaType   = "application/vnd.onmetal.image.vmlinuz.v1alpha1.vmlinuz"
)

type Metadata struct {
	// Name is the name of the image. Required.
	Name string `json:"name"`
}

type LayeredStore struct {
	*content.Memorystore
	provider containerdcontent.Provider
}

func NewLayeredStore(provider containerdcontent.Provider) *LayeredStore {
	return &LayeredStore{
		Memorystore: content.NewMemoryStore(),
		provider:    provider,
	}
}

func (s *LayeredStore) ReaderAt(ctx context.Context, desc imagespecv1.Descriptor) (containerdcontent.ReaderAt, error) {
	readerAt, err := s.Memorystore.ReaderAt(ctx, desc)
	if err == nil {
		return readerAt, nil
	}
	if s.provider != nil {
		return s.provider.ReaderAt(ctx, desc)
	}
	return nil, err
}

func addImageLayers(store *content.FileStore, rootFSPath, initRAMFSPath, kernelPath string) ([]imagespecv1.Descriptor, error) {
	parts := []struct {
		MediaType string
		Path      string
	}{
		{
			MediaType: RootFSLayerMediaType,
			Path:      rootFSPath,
		},
		{
			MediaType: InitRAMFSLayerMediaType,
			Path:      initRAMFSPath,
		},
		{
			MediaType: VMLinuzLayerMediaType,
			Path:      kernelPath,
		},
	}
	var descriptors []imagespecv1.Descriptor
	for _, part := range parts {
		descriptor, err := store.Add("", part.MediaType, part.Path)
		if err != nil {
			return nil, fmt.Errorf("could not add %s from %s: %w", part.MediaType, part.Path, err)
		}

		log.Println("Successfully added", part.MediaType, "with digest", descriptor.Digest)
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

type packedImage struct {
	Provider         containerdcontent.Provider
	ConfigDescriptor imagespecv1.Descriptor
	LayerDescriptors []imagespecv1.Descriptor
}

func packImage(store *content.FileStore, imageName string, rootFSPath, initRAMFSPath, kernelPath string) (*packedImage, error) {
	layerDescriptors, err := addImageLayers(store, rootFSPath, initRAMFSPath, kernelPath)
	if err != nil {
		return nil, fmt.Errorf("error adding image layers: %w", err)
	}

	metadata := &Metadata{
		Name: imageName,
	}
	configData, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("error marshalling image metadata: %w", err)
	}

	layered := NewLayeredStore(store)
	configDescriptor := layered.Add("", ConfigMediaType, configData)

	return &packedImage{
		Provider:         layered,
		ConfigDescriptor: configDescriptor,
		LayerDescriptors: layerDescriptors,
	}, nil
}

func pushImageBundle(ctx context.Context, ref string, bundle *packedImage) (imagespecv1.Descriptor, error) {
	authClient, err := dockerauth.NewClient()
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("could not create auth client: %w", err)
	}

	resolver, err := authClient.ResolverWithOpts()
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("could not create resolver: %w", err)
	}

	// We set this up to disable nasty warnings due to unknown layer types
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, ConfigMediaType, "config-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, RootFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, InitRAMFSLayerMediaType, "layer-")
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, VMLinuzLayerMediaType, "layer-")

	res, err := oras.Push(ctx, resolver, ref, bundle.Provider, bundle.LayerDescriptors,
		oras.WithConfig(bundle.ConfigDescriptor),
		oras.WithPushBaseHandler(images.HandlerFunc(func(ctx context.Context, desc imagespecv1.Descriptor) (subdescs []imagespecv1.Descriptor, err error) {
			log.Println("Uploading", desc.MediaType, "with digest", desc.Digest.Encoded()[:12])
			return nil, nil
		})),
		oras.WithNameValidation(nil),
	)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error pushing to %s: %w", ref, err)
	}

	return res, nil
}

func run(ctx context.Context, imageName, ref, rootFSPath, initRAMFSPath, kernelPath string) error {
	if imageName == "" {
		imageName = path.Base(ref)
		if idx := strings.Index(imageName, ":"); idx != -1 {
			imageName = imageName[:idx]
		}
	}

	store := content.NewFileStore(".")
	defer func() { _ = store.Close() }()

	bundle, err := packImage(store, imageName, rootFSPath, initRAMFSPath, kernelPath)
	if err != nil {
		return fmt.Errorf("could not build image: %w", err)
	}

	res, err := pushImageBundle(ctx, ref, bundle)
	if err != nil {
		return fmt.Errorf("error pushing image bundle: %w", err)
	}

	log.Println("Successfully pushed digest", res.Digest, "to ref", ref)
	return nil
}

func main() {
	ctx, cancel := signalcontext.OnInterrupt()
	defer cancel()

	var (
		imageName     string
		rootFSPath    string
		initRAMFSPath string
		kernelPath    string
	)

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "%s [options] <ref>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&imageName, "image-name", "", "Name of the image to push. If unspecified, it will be inferred from the ref.")
	flag.StringVar(&rootFSPath, "root-fs-file", "", "Path pointing to a root fs file.")
	flag.StringVar(&initRAMFSPath, "initram-fs-file", "", "Path pointing to an initram fs file.")
	flag.StringVar(&kernelPath, "kernel-file", "", "Path pointing to a kernel file (usually ending with 'vmlinuz').")

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	ref := flag.Arg(0)

	if err := run(ctx, imageName, ref, rootFSPath, initRAMFSPath, kernelPath); err != nil {
		log.Fatalln("Error running:", err)
	}
}
