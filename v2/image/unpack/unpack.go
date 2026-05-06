// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package unpack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

type Unpacker struct {
	artifactType string

	fetcher content.Fetcher

	configPart            *Part
	layerParts            []*Part
	layerPartsByMediaType map[string]*Part
}

type Part struct {
	MediaTypes []string
	Value      Value
}

func NewUnpacker(fetcher content.Fetcher) *Unpacker {
	return &Unpacker{
		fetcher:               fetcher,
		layerPartsByMediaType: make(map[string]*Part),
	}
}

func (i *Unpacker) ArtifactType(artifactType string) *Unpacker {
	i.artifactType = artifactType
	return i
}

type jsonValue struct {
	v any
}

func (v *jsonValue) Unpack(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) error {
	data, err := content.FetchAll(ctx, fetcher, desc)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v.v)
}

func newJSONValue(v any) *jsonValue {
	return &jsonValue{v: v}
}

type descriptorValue ocispec.Descriptor

func newDescriptorValue(p *ocispec.Descriptor) *descriptorValue {
	return (*descriptorValue)(p)
}

func (d *descriptorValue) Unpack(_ context.Context, _ content.Fetcher, desc ocispec.Descriptor) error {
	*d = descriptorValue(desc)
	return nil
}

type descriptorSliceValue []ocispec.Descriptor

func newDescriptorSliceValue(p *[]ocispec.Descriptor) *descriptorSliceValue {
	return (*descriptorSliceValue)(p)
}

func (v *descriptorSliceValue) Unpack(_ context.Context, _ content.Fetcher, desc ocispec.Descriptor) error {
	*v = append(*v, desc)
	return nil
}

type Value interface {
	Unpack(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) error
}

type Completer interface {
	Complete() error
}

func (i *Unpacker) ConfigVar(value Value, mediaTypes []string) *Unpacker {
	if i.configPart != nil {
		panic("config already registered")
	}
	i.configPart = &Part{
		MediaTypes: mediaTypes,
		Value:      value,
	}
	return i
}

func (i *Unpacker) Config(dst *ocispec.Descriptor, mediaTypes []string) *Unpacker {
	return i.ConfigVar(newDescriptorValue(dst), mediaTypes)
}

func (i *Unpacker) ConfigJSON(v any, mediaTypes []string) *Unpacker {
	return i.ConfigVar(newJSONValue(v), mediaTypes)
}

func (i *Unpacker) LayerVar(value Value, mediaTypes []string) *Unpacker {
	part := &Part{
		MediaTypes: mediaTypes,
		Value:      value,
	}

	i.layerParts = append(i.layerParts, part)
	for _, mediaType := range mediaTypes {
		if _, ok := i.layerPartsByMediaType[mediaType]; ok {
			panic(fmt.Sprintf("media type %q already registered", mediaType))
		}

		i.layerPartsByMediaType[mediaType] = part
	}

	return i
}

func (i *Unpacker) LayerJSON(v any, mediaTypes []string) *Unpacker {
	return i.LayerVar(newJSONValue(v), mediaTypes)
}

func (i *Unpacker) Layer(dst *ocispec.Descriptor, mediaTypes []string) *Unpacker {
	return i.LayerVar(newDescriptorValue(dst), mediaTypes)
}

func (i *Unpacker) LayerSlice(dst *[]ocispec.Descriptor, mediaTypes []string) *Unpacker {
	return i.LayerVar(newDescriptorSliceValue(dst), mediaTypes)
}

func (i *Unpacker) Unpack(ctx context.Context, desc ocispec.Descriptor) error {
	data, err := content.FetchAll(ctx, i.fetcher, desc)
	if err != nil {
		return err
	}

	manifest := &ocispec.Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return err
	}

	if manifest.MediaType != ocispec.MediaTypeImageManifest {
		return fmt.Errorf("%w: not an image manifest: %q", errdef.ErrUnsupported, manifest.MediaType)
	}

	if i.artifactType != manifest.ArtifactType {
		return fmt.Errorf("%w: artifact type %q (expected %q)", errdef.ErrUnsupported, manifest.ArtifactType, i.artifactType)
	}

	var errs []error
	if i.configPart != nil {
		if !slices.Contains(i.configPart.MediaTypes, manifest.Config.MediaType) {
			errs = append(errs, fmt.Errorf("unknown config media type %q", manifest.Config.MediaType))
		} else {
			if err := i.configPart.Value.Unpack(ctx, i.fetcher, manifest.Config); err != nil {
				errs = append(errs, err)
			}
		}
	}

	for _, desc := range manifest.Layers {
		part, ok := i.layerPartsByMediaType[desc.MediaType]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown layer media type %q", desc.MediaType))
		} else {
			if err := part.Value.Unpack(ctx, i.fetcher, desc); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
