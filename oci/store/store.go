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

package store

import (
	"context"
	"fmt"

	"github.com/onmetal/onmetal-image/oci/descriptormatcher"

	"github.com/onmetal/onmetal-image/oci/image"

	"github.com/distribution/distribution/reference"
	"github.com/onmetal/onmetal-image/oci/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Store struct {
	layout *layout.Layout
}

func (s *Store) Put(ctx context.Context, img image.Image) error {
	desc := ocispec.Descriptor{MediaType: img.Descriptor().MediaType, Digest: img.Descriptor().Digest}
	if err := s.layout.ReplaceImage(ctx, img, descriptormatcher.Equal(desc)); err != nil {
		return fmt.Errorf("could not create image: %w", err)
	}
	return nil
}

func (s *Store) Push(ctx context.Context, ref string, img image.Image) error {
	if err := s.Put(ctx, img); err != nil {
		return fmt.Errorf("error putting image: %w", err)
	}
	if err := s.Tag(ctx, img.Descriptor().Digest.String(), ref); err != nil {
		return fmt.Errorf("error tagging image with ref %s: %w", ref, err)
	}
	return nil
}

func (s *Store) referenceToMatcher(ref string) (descriptormatcher.Matcher, error) {
	r, err := reference.ParseAnyReference(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid ref: %w", err)
	}

	var matchers []descriptormatcher.Matcher
	if digested, ok := r.(reference.Digested); ok {
		matchers = append(matchers, descriptormatcher.Digests(digested.Digest()))
	}

	if named, ok := r.(reference.Named); ok {
		name := named.Name()
		if tagged, ok := named.(reference.Tagged); ok {
			name = fmt.Sprintf("%s:%s", name, tagged.Tag())
		}

		matchers = append(matchers, descriptormatcher.Name(name))
	}

	if len(matchers) == 0 {
		return nil, fmt.Errorf("could not construct matchers from ref %s", ref)
	}
	return descriptormatcher.And(matchers...), nil
}

func (s *Store) Delete(ctx context.Context, ref string) error {
	match, err := s.referenceToMatcher(ref)
	if err != nil {
		return err
	}

	if err := s.layout.Indexer().Delete(ctx, match); err != nil {
		return fmt.Errorf("error deleting ref %s from indexer: %w", ref, err)
	}
	return nil
}

func (s *Store) resolveDescriptor(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	match, err := s.referenceToMatcher(ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	desc, err := s.layout.Indexer().Find(ctx, match)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error getting descriptor for ref %s: %w", ref, err)
	}
	return desc, nil
}

func (s *Store) Resolve(ctx context.Context, ref string) (image.Image, error) {
	desc, err := s.resolveDescriptor(ctx, ref)
	if err != nil {
		return nil, err
	}

	return s.layout.Image(ctx, desc)
}

func (s *Store) Tag(ctx context.Context, srcRef, dstRef string) error {
	if _, err := reference.ParseNamed(dstRef); err != nil {
		return fmt.Errorf("destination has to be a named reference: %w", err)
	}

	srcDesc, err := s.resolveDescriptor(ctx, srcRef)
	if err != nil {
		return fmt.Errorf("error resolving source ref: %w", err)
	}

	dstDesc := ocispec.Descriptor{
		MediaType: srcDesc.MediaType,
		Digest:    srcDesc.Digest,
		Size:      srcDesc.Size,
		Platform:  srcDesc.Platform,
		Annotations: map[string]string{
			ocispec.AnnotationRefName: dstRef,
		},
	}

	if err := s.layout.Indexer().Replace(ctx, dstDesc, descriptormatcher.Name(dstRef)); err != nil {
		return fmt.Errorf("error indexing ref descriptor: %w", err)
	}
	return nil
}

func (s *Store) Untag(ctx context.Context, ref string) error {
	if _, err := reference.ParseNamed(ref); err != nil {
		return fmt.Errorf("ref has to be a named reference: %w", err)
	}
	if err := s.layout.Indexer().Delete(ctx, descriptormatcher.Name(ref)); err != nil {
		return fmt.Errorf("error removing index entries: %w", err)
	}
	return nil
}

func (s *Store) Layout() *layout.Layout {
	return s.layout
}

func New(path string) (*Store, error) {
	l, err := layout.New(path)
	if err != nil {
		return nil, fmt.Errorf("could not created oci layout: %w", err)
	}
	return &Store{layout: l}, nil
}
