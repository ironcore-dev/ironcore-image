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

package contentutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/opencontainers/go-digest"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func DigestFromFile(filename string) (digest.Digest, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer func() { _ = f.Close() }()

	return digest.FromReader(f)
}

func DigestFromJSONEncodedValue(v interface{}) (digest.Digest, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("error converting %v to json: %w", v, err)
	}

	return digest.FromBytes(data), nil
}

type DescriptorOption func(descriptor *imagespecv1.Descriptor)

func WithMediaType(mediaType string) DescriptorOption {
	return func(descriptor *imagespecv1.Descriptor) {
		descriptor.MediaType = mediaType
	}
}

func DescriptorFromFile(filename string, opts ...DescriptorOption) (imagespecv1.Descriptor, error) {
	f, err := os.Open(filename)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error opening %s: %w", filename, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error stat %s: %w", filename, err)
	}

	dgst, err := digest.FromReader(f)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error computing digest of %s: %w", filename, err)
	}

	desc := imagespecv1.Descriptor{
		Digest: dgst,
		Size:   info.Size(),
	}

	for _, opt := range opts {
		opt(&desc)
	}
	return desc, nil
}

func DescriptorFromJSONEncodedValue(v interface{}, opts ...DescriptorOption) (imagespecv1.Descriptor, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("could not convert %v to json: %w", v, err)
	}

	return DescriptorFromData(data), nil
}

func DescriptorFromData(data []byte, opts ...DescriptorOption) imagespecv1.Descriptor {
	desc := imagespecv1.Descriptor{
		Digest: digest.FromBytes(data),
		Size:   int64(len(data)),
	}
	for _, opt := range opts {
		opt(&desc)
	}
	return desc
}

func WriteFileToIngester(ctx context.Context, ing content.Ingester, filename string, opts ...DescriptorOption) (imagespecv1.Descriptor, error) {
	desc, err := DescriptorFromFile(filename, opts...)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error creating file descriptor: %w", err)
	}

	ref := remotes.MakeRefKey(ctx, desc)

	f, err := os.Open(filename)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error opening file %s: %w", filename, err)
	}
	defer func() { _ = f.Close() }()

	if err := content.WriteBlob(ctx, ing, ref, f, desc); err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error writing file %s: %w", filename, err)
	}
	return desc, nil
}

func WriteJSONEncodedValueToIngester(ctx context.Context, ing content.Ingester, v interface{}, opts ...DescriptorOption) (imagespecv1.Descriptor, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("could not convert %v to json: %w", v, err)
	}

	return WriteDataToIngester(ctx, ing, data, opts...)
}

func WriteDataToIngester(ctx context.Context, ing content.Ingester, data []byte, opts ...DescriptorOption) (imagespecv1.Descriptor, error) {
	desc := DescriptorFromData(data, opts...)

	ref := remotes.MakeRefKey(ctx, desc)

	if err := content.WriteBlob(ctx, ing, ref, bytes.NewReader(data), desc); err != nil {
		return imagespecv1.Descriptor{}, fmt.Errorf("error writing data: %w", err)
	}
	return desc, nil
}
