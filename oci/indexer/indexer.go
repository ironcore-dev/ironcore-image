// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironcore-dev/ironcore-image/oci/descriptormatcher"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const Filename = "index.json"

var ErrNotFound = errors.New("not found")

type Indexer struct {
	path string
}

func (f *Indexer) readIndex() (*ocispec.Index, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("error reading index file: %w", err)
	}

	index := &ocispec.Index{}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, fmt.Errorf("error reading index: %w", err)
	}

	return index, nil
}

func (f *Indexer) writeIndex(index *ocispec.Index) error {
	data, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("could not convert index to json: %w", err)
	}

	if err := os.WriteFile(f.path, data, 0666); err != nil {
		return fmt.Errorf("error writing index: %w", err)
	}

	return nil
}

func (f *Indexer) Add(ctx context.Context, desc ocispec.Descriptor) error {
	index, err := f.readIndex()
	if err != nil {
		return err
	}

	index.Manifests = append(index.Manifests, desc)

	return f.writeIndex(index)
}

func (f *Indexer) Find(ctx context.Context, match descriptormatcher.Matcher) (ocispec.Descriptor, error) {
	index, err := f.readIndex()
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	for _, manifest := range index.Manifests {
		if match(manifest) {
			return manifest, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("%w: no index matching", ErrNotFound)
}

func (f *Indexer) List(ctx context.Context, match descriptormatcher.Matcher) ([]ocispec.Descriptor, error) {
	index, err := f.readIndex()
	if err != nil {
		return nil, err
	}

	var res []ocispec.Descriptor
	for _, desc := range index.Manifests {
		if match(desc) {
			res = append(res, desc)
		}
	}
	return res, nil
}

func (f *Indexer) Replace(ctx context.Context, desc ocispec.Descriptor, match descriptormatcher.Matcher) error {
	index, err := f.readIndex()
	if err != nil {
		return err
	}

	var remaining []ocispec.Descriptor
	for _, manifest := range index.Manifests {
		if !match(manifest) {
			remaining = append(remaining, manifest)
		}
	}

	index.Manifests = append(remaining, desc)
	if err := f.writeIndex(index); err != nil {
		return err
	}
	return nil
}

func (f *Indexer) Delete(ctx context.Context, match descriptormatcher.Matcher) error {
	index, err := f.readIndex()
	if err != nil {
		return err
	}

	var remaining []ocispec.Descriptor
	for _, manifest := range index.Manifests {
		if !match(manifest) {
			remaining = append(remaining, manifest)
		}
	}

	index.Manifests = remaining
	if err := f.writeIndex(index); err != nil {
		return err
	}
	return nil
}

func New(path string) (*Indexer, error) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating base directory: %w", err)
	}

	indexer := &Indexer{
		path: path,
	}
	if _, err := indexer.readIndex(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error checking for index: %w", err)
		}

		if err := indexer.writeIndex(&ocispec.Index{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Manifests: make([]ocispec.Descriptor, 0),
		}); err != nil {
			return nil, fmt.Errorf("error writing initial index: %w", err)
		}
	}
	return indexer, nil
}
