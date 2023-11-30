// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

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
