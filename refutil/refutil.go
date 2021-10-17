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

package refutil

import (
	"fmt"

	"github.com/distribution/distribution/reference"
	"github.com/onmetal/onmetal-image/annotations"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func copyStringStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	res := make(map[string]string, len(m))
	for k, v := range m {
		res[k] = v
	}
	return res
}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	res := make([]string, len(s))
	copy(res, s)
	return res
}

func CopyDescriptor(desc v1.Descriptor) v1.Descriptor {
	return v1.Descriptor{
		MediaType:   desc.MediaType,
		Digest:      desc.Digest,
		Size:        desc.Size,
		URLs:        copyStringSlice(desc.URLs),
		Annotations: copyStringStringMap(desc.Annotations),
	}
}

func DescriptorWithReference(desc v1.Descriptor, ref reference.Named) (v1.Descriptor, error) {
	res := CopyDescriptor(desc)
	delete(res.Annotations, annotations.ImageName)
	delete(res.Annotations, v1.AnnotationRefName)

	if res.Annotations == nil {
		res.Annotations = make(map[string]string)
	}
	res.Annotations[annotations.ImageName] = ref.Name()
	if tagged, ok := ref.(reference.Tagged); ok {
		res.Annotations[v1.AnnotationRefName] = tagged.Tag()
	}
	return res, nil
}

func ReferenceFromDescriptor(desc v1.Descriptor) (reference.Named, error) {
	name, ok1 := desc.Annotations[annotations.ImageName]
	tag, ok2 := desc.Annotations[v1.AnnotationRefName]
	if !ok1 && !ok2 {
		return nil, nil
	}
	if !ok1 {
		return nil, fmt.Errorf("no name but tag %s is set", tag)
	}
	ref, err := reference.WithName(name)
	if err != nil {
		return nil, err
	}

	if ok2 {
		ref, err = reference.WithTag(ref, tag)
		if err != nil {
			return nil, err
		}
	}

	return ref, nil
}

func ReferenceMatchesDescriptor(ref reference.Reference, desc v1.Descriptor) (bool, error) {
	digested, hasDigest := ref.(reference.Digested)
	named, hasName := ref.(reference.Named)
	tagged, hasTag := ref.(reference.Tagged)

	if hasDigest && desc.Digest != digested.Digest() {
		return false, nil
	}

	descRef, err := ReferenceFromDescriptor(desc)
	if err != nil {
		return false, err
	}

	if descRef == nil {
		return !hasName && !hasTag, nil
	}

	if hasName && descRef.Name() != named.Name() {
		return false, nil
	}

	if hasTag {
		descTagged, ok := descRef.(reference.Tagged)
		return ok && descTagged.Tag() == tagged.Tag(), nil
	}

	return true, nil
}

func TrimDescriptorReferenceAnnotations(desc v1.Descriptor, ref reference.Reference) v1.Descriptor {
	res := CopyDescriptor(desc)
	if _, ok := ref.(reference.Tagged); !ok {
		delete(res.Annotations, v1.AnnotationRefName)
	}
	if _, ok := ref.(reference.Named); !ok {
		delete(res.Annotations, annotations.ImageName)
	}
	return res
}
