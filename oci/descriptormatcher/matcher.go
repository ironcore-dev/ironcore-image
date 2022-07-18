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

package descriptormatcher

import (
	"reflect"
	"strings"

	"github.com/onmetal/onmetal-image/utils/sets"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Matcher func(descriptor ocispec.Descriptor) bool

func And(matchers ...Matcher) Matcher {
	return func(descriptor ocispec.Descriptor) bool {
		for _, matcher := range matchers {
			if !matcher(descriptor) {
				return false
			}
		}
		return true
	}
}

func Or(matchers ...Matcher) Matcher {
	return func(descriptor ocispec.Descriptor) bool {
		for _, matcher := range matchers {
			if matcher(descriptor) {
				return true
			}
		}
		return false
	}
}

func Equal(to ocispec.Descriptor) Matcher {
	return func(descriptor ocispec.Descriptor) bool {
		return reflect.DeepEqual(to, descriptor)
	}
}

func Every(ocispec.Descriptor) bool {
	return true
}

func None(descriptor ocispec.Descriptor) bool {
	return false
}

func Annotation(key, value string) Matcher {
	return func(descriptor ocispec.Descriptor) bool {
		actual, ok := descriptor.Annotations[key]
		return ok && actual == value
	}
}

func MediaTypes(mediaTypes ...string) Matcher {
	s := sets.New[string](mediaTypes...)
	return func(descriptor ocispec.Descriptor) bool {
		return s.Has(descriptor.MediaType)
	}
}

func Digests(digests ...digest.Digest) Matcher {
	s := sets.New[string]()
	for _, d := range digests {
		s.Insert(string(d))
	}
	return func(descriptor ocispec.Descriptor) bool {
		return s.Has(string(descriptor.Digest))
	}
}

func Name(name string) Matcher {
	return Annotation(ocispec.AnnotationRefName, name)
}

func EncodedDigestPrefix(prefix string) Matcher {
	return func(descriptor ocispec.Descriptor) bool {
		return strings.HasPrefix(descriptor.Digest.Encoded(), prefix)
	}
}
