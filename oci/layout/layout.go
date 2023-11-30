// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ocicontent "github.com/ironcore-dev/ironcore-image/oci/content"
	"github.com/ironcore-dev/ironcore-image/oci/descriptormatcher"
	ociimage "github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/ironcore-dev/ironcore-image/oci/indexer"
	"github.com/ironcore-dev/ironcore-image/oci/local"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Layout struct {
	store   *local.Store
	indexer *indexer.Indexer
}

// AddImage adds an image to the layout.
func (l *Layout) AddImage(ctx context.Context, image ociimage.Image) error {
	if err := ocicontent.WriteImageToIngester(ctx, l.store, image); err != nil {
		return fmt.Errorf("error writing image: %w", err)
	}

	if err := l.indexer.Add(ctx, image.Descriptor()); err != nil {
		return fmt.Errorf("error adding image %s to index: %w", image.Descriptor().Digest, err)
	}
	return nil
}

// ReplaceImage replaces the target image with the new one.
func (l *Layout) ReplaceImage(ctx context.Context, image ociimage.Image, match descriptormatcher.Matcher) error {
	if err := ocicontent.WriteImageToIngester(ctx, l.store, image); err != nil {
		return fmt.Errorf("error writing image: %w", err)
	}

	if err := l.indexer.Replace(ctx, image.Descriptor(), match); err != nil {
		return fmt.Errorf("error adding image %s to index: %w", image.Descriptor().Digest, err)
	}
	return nil
}

// Image returns the image for the given descriptor.
func (l *Layout) Image(ctx context.Context, desc ocispec.Descriptor) (ociimage.Image, error) {
	desc, err := l.indexer.Find(ctx, descriptormatcher.Equal(desc))
	if err != nil {
		return nil, fmt.Errorf("could not find descriptor in index: %w", err)
	}

	return ocicontent.Image(l.store, desc), nil
}

// Images lists all images.
func (l *Layout) Images(ctx context.Context) ([]ociimage.Image, error) {
	descs, err := l.indexer.List(ctx, descriptormatcher.Every)
	if err != nil {
		return nil, err
	}

	res := make([]ociimage.Image, 0, len(descs))
	for _, desc := range descs {
		res = append(res, ocicontent.Image(l.store, desc))
	}
	return res, nil
}

// Indexer returns the indexer.Indexer of the oci layout.
func (l *Layout) Indexer() *indexer.Indexer {
	return l.indexer
}

// Store returns the backing local.Store of the oci layout.
func (l *Layout) Store() *local.Store {
	return l.store
}

const ociLayoutContent = `{"imageLayoutVersion":"1.0.0"}`

// New returns a new oci layout.
func New(path string) (*Layout, error) {
	store, err := local.NewStore(path)
	if err != nil {
		return nil, fmt.Errorf("error creating store: %w", err)
	}

	index, err := indexer.New(filepath.Join(path, indexer.Filename))
	if err != nil {
		return nil, fmt.Errorf("error creating indexer: %w", err)
	}

	if err := os.WriteFile(filepath.Join(path, "oci-layout"), []byte(ociLayoutContent), 0666); err != nil {
		return nil, fmt.Errorf("error writing oci layout: %w", err)
	}

	return &Layout{
		indexer: index,
		store:   store,
	}, nil
}
