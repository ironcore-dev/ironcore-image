// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xoci

import (
	"context"
	"errors"

	"github.com/ironcore-dev/ironcore-image/v2/xcontent"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

var (
	ErrSkipAll   = errors.New("skip all")
	ErrSkipIndex = errors.New("skip index")
)

func Walk(
	ctx context.Context,
	s content.Fetcher,
	desc ocispec.Descriptor,
	f func(ctx context.Context, desc ocispec.Descriptor, err error) error,
) error {
	if err := walk(ctx, s, desc, f); err != nil {
		if errors.Is(err, ErrSkipAll) || errors.Is(err, ErrSkipIndex) {
			return nil
		}
		return err
	}
	return nil
}

func walk(
	ctx context.Context,
	s content.Fetcher,
	desc ocispec.Descriptor,
	f func(ctx context.Context, desc ocispec.Descriptor, err error) error,
) error {
	if desc.MediaType != ocispec.MediaTypeImageIndex {
		return f(ctx, desc, nil)
	}

	index := &ocispec.Index{}
	err := xcontent.FetchJSON(ctx, s, desc, index)
	err1 := f(ctx, desc, err)
	if err != nil || err1 != nil {
		return err1
	}

	for _, desc := range index.Manifests {
		if err := walk(ctx, s, desc, f); err != nil {
			if desc.MediaType != ocispec.MediaTypeImageIndex || !errors.Is(err, ErrSkipIndex) {
				return err
			}
		}
	}
	return nil
}
