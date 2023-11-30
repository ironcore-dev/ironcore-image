// Copyright 2021 IronCore authors
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

package sets

type Empty struct{}

type Set[E comparable] map[E]Empty

func New[E comparable](items ...E) Set[E] {
	s := make(Set[E])
	s.Insert(items...)
	return s
}

func (s Set[E]) Insert(items ...E) {
	for _, item := range items {
		s[item] = Empty{}
	}
}

func (s Set[E]) Has(item E) bool {
	_, ok := s[item]
	return ok
}

func (s Set[E]) Delete(items ...E) {
	for _, item := range items {
		delete(s, item)
	}
}
