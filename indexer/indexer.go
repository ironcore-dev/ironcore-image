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

package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/onmetal/onmetal-image/refutil"

	"github.com/opencontainers/go-digest"

	"github.com/distribution/distribution/reference"

	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const indexFilename = "index.json"

var ErrNotFound = errors.New("not found")

type Indexer interface {
	Index(ctx context.Context, desc ocispec.Descriptor) error
	GetByReference(ctx context.Context, ref reference.Reference, opts ...GetByReferenceOption) (ocispec.Descriptor, error)
	List(ctx context.Context, opts ...ListOption) ([]ocispec.Descriptor, error)
}

type fileIndexer struct {
	root string
}

func (f *fileIndexer) indexPath() string {
	return filepath.Join(f.root, indexFilename)
}

func (f *fileIndexer) readIndex() (*ocispec.Index, error) {
	data, err := os.ReadFile(f.indexPath())
	if err != nil {
		return nil, fmt.Errorf("error reading index file: %w", err)
	}

	index := &ocispec.Index{}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, fmt.Errorf("error reading index: %w", err)
	}

	return index, nil
}

func (f *fileIndexer) writeIndex(index *ocispec.Index) error {
	data, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("could not convert index to json: %w", err)
	}

	if err := os.WriteFile(f.indexPath(), data, 0666); err != nil {
		return fmt.Errorf("error writing index: %w", err)
	}

	return nil
}

func descriptorTargetIndex(index *ocispec.Index, desc ocispec.Descriptor) (int, error) {
	for i, manifest := range index.Manifests {
		ok, err := shouldOverwrite(manifest, desc)
		if err != nil {
			return 0, err
		}

		if ok {
			return i, nil
		}
	}
	return -1, nil
}

func shouldOverwrite(d1, d2 ocispec.Descriptor) (bool, error) {
	if d1.MediaType != d2.MediaType {
		return false, nil
	}

	r1, err := refutil.ReferenceFromDescriptor(d1)
	if err != nil {
		return false, err
	}

	r2, err := refutil.ReferenceFromDescriptor(d2)
	if err != nil {
		return false, err
	}

	switch {
	// The references are not equal.
	case r1 != r2:
		return false, nil
	// Both references are equal and non-nil, overwrite.
	case r1 != nil:
		return true, nil
	// Both references are nil, compare by descriptor digest.
	default:
		return d1.Digest == d2.Digest, nil
	}
}

type GetByReferenceOption interface {
	ApplyToGetByReferenceOptions(o *GetByReferenceOptions)
}

type GetByReferenceOptions struct {
	MediaType string
}

func (o *GetByReferenceOptions) ApplyOptions(opts []GetByReferenceOption) {
	for _, opt := range opts {
		opt.ApplyToGetByReferenceOptions(o)
	}
}

func (o *GetByReferenceOptions) ApplyToGetByReferenceOptions(o2 *GetByReferenceOptions) {
	if o.MediaType != "" {
		o2.MediaType = o.MediaType
	}
}

func NewGetByReferenceOptions() *GetByReferenceOptions {
	return &GetByReferenceOptions{}
}

func (f *fileIndexer) GetByReference(ctx context.Context, ref reference.Reference, opts ...GetByReferenceOption) (ocispec.Descriptor, error) {
	o := NewGetByReferenceOptions()
	o.ApplyOptions(opts)

	index, err := f.readIndex()
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	for _, desc := range index.Manifests {
		if !isSupportedDescriptor(desc) {
			continue
		}
		if o.MediaType != "" && desc.MediaType != o.MediaType {
			continue
		}

		ok, err := refutil.ReferenceMatchesDescriptor(ref, desc)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if ok {
			return refutil.TrimDescriptorReferenceAnnotations(desc, ref), nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("%w: no descriptor matches ref %s", ErrNotFound, ref)
}

func (f *fileIndexer) Index(ctx context.Context, desc ocispec.Descriptor) error {
	if err := checkDescriptor(desc); err != nil {
		return err
	}

	index, err := f.readIndex()
	if err != nil {
		return err
	}

	idx, err := descriptorTargetIndex(index, desc)
	if err != nil {
		return fmt.Errorf("error determining descriptor index: %w", err)
	}

	if idx != -1 {
		index.Manifests[idx] = desc
	} else {
		index.Manifests = append(index.Manifests, desc)
	}
	return f.writeIndex(index)
}

// isSupportedDescriptor checks if the descriptor has a supported media type.
// Unsupported media types encountered in an ocispec.Index should be skipped.
// Unsupported media types in an argument to this indexer cause an error, see checkDescriptor.
func isSupportedDescriptor(desc ocispec.Descriptor) bool {
	return desc.MediaType == ocispec.MediaTypeImageManifest
}

// checkDescriptor checks if the descriptor has a supported media type and if not returns an error.
// This should *only* be used for arguments to functions of this indexer, as unsupported media types
// in an underlying ocispec.Index should just be ignored, see isSupportedDescriptor.
func checkDescriptor(desc ocispec.Descriptor) error {
	if !isSupportedDescriptor(desc) {
		return fmt.Errorf("unsupported descriptor media type %q", desc.MediaType)
	}
	return nil
}

type ListOptions struct {
	MediaType string
}

func NewListOptions() *ListOptions {
	return &ListOptions{}
}

func (o *ListOptions) ApplyToListOptions(o2 *ListOptions) {
	if o.MediaType != "" {
		o2.MediaType = o.MediaType
	}
}

func (o *ListOptions) ApplyOptions(opts []ListOption) {
	for _, opt := range opts {
		opt.ApplyToListOptions(o)
	}
}

type ListOption interface {
	ApplyToListOptions(o *ListOptions)
}

type WithMediaType string

func (w WithMediaType) ApplyToGetByReferenceOptions(o *GetByReferenceOptions) {
	o.MediaType = string(w)
}

func (w WithMediaType) ApplyToListOptions(o *ListOptions) {
	o.MediaType = string(w)
}

func (f *fileIndexer) List(ctx context.Context, opts ...ListOption) ([]ocispec.Descriptor, error) {
	o := NewListOptions()
	o.ApplyOptions(opts)

	index, err := f.readIndex()
	if err != nil {
		return nil, err
	}

	var res []ocispec.Descriptor
	for _, desc := range index.Manifests {
		if !isSupportedDescriptor(desc) {
			continue
		}
		if o.MediaType != "" && o.MediaType != desc.MediaType {
			continue
		}

		res = append(res, desc)
	}
	return res, nil
}

// canonicalDigestPartRegex is a regex that is similar to the regex of digest.Canonical, except
// it accepts at least one and at most len(regex(digest.Canonical)) - 1.
var canonicalDigestPartRegex = regexp.MustCompile(`^[a-f0-9]{1,63}$`)

func ResolveFuzzyRef(ctx context.Context, indexer Indexer, fuzzyRef string, opts ...GetByReferenceOption) (ocispec.Descriptor, error) {
	o := NewGetByReferenceOptions()
	o.ApplyOptions(opts)

	ref, err := reference.Parse(fuzzyRef)
	if err == nil {
		if desc, err := indexer.GetByReference(ctx, ref, o); err == nil {
			return desc, nil
		}
	}

	ref, err = reference.ParseAnyReference(fuzzyRef)
	if err == nil {
		if desc, err := indexer.GetByReference(ctx, ref, o); err == nil {
			return desc, nil
		}
	}

	if !canonicalDigestPartRegex.MatchString(fuzzyRef) {
		return ocispec.Descriptor{}, fmt.Errorf("%w: no match for %s", ErrNotFound, fuzzyRef)
	}

	listOpts := NewListOptions()
	listOpts.MediaType = o.MediaType
	descs, err := indexer.List(ctx, listOpts)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error listing descriptors: %w", err)
	}
	for _, desc := range descs {
		if desc.Digest.Algorithm() == digest.Canonical && strings.HasPrefix(desc.Digest.Encoded(), fuzzyRef) {
			return desc, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("%w: no match for fuzzy reference %s", ErrNotFound, fuzzyRef)
}

func New(root string) (Indexer, error) {
	if err := os.MkdirAll(root, 0777); err != nil {
		return nil, fmt.Errorf("error creating base directory: %w", err)
	}

	indexer := &fileIndexer{
		root: root,
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
