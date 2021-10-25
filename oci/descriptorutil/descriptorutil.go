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

package descriptorutil

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

func Copy(desc ocispec.Descriptor) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType:   desc.MediaType,
		Digest:      desc.Digest,
		Size:        desc.Size,
		URLs:        copyStringSlice(desc.URLs),
		Annotations: copyStringStringMap(desc.Annotations),
	}
}

func WithAnnotations(desc ocispec.Descriptor, annotations map[string]string) ocispec.Descriptor {
	res := Copy(desc)
	if res.Annotations == nil && len(annotations) > 0 {
		res.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		res.Annotations[k] = v
	}
	return res
}

func Plain(desc ocispec.Descriptor) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: desc.MediaType,
		Digest:    desc.Digest,
		Size:      desc.Size,
		Platform:  desc.Platform,
	}
}

func WithName(desc ocispec.Descriptor, name string) ocispec.Descriptor {
	return WithAnnotations(desc, map[string]string{
		ocispec.AnnotationRefName: name,
	})
}
