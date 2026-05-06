// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package orderedmap

import (
	"iter"

	"github.com/ironcore-dev/ironcore-image/v2/xcontainer/xlist"
)

type entry[K comparable, V any] struct {
	node  *xlist.Element[K]
	value V
}

type OrderedMap[K comparable, V any] struct {
	entries map[K]*entry[K, V]
	order   *xlist.List[K]
}

func New[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		entries: make(map[K]*entry[K, V]),
		order:   xlist.New[K](),
	}
}

func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	e, ok := m.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (m *OrderedMap[K, V]) Value(key K) V {
	v, _ := m.Get(key)
	return v
}

func (m *OrderedMap[K, V]) Set(key K, value V) {
	if e, ok := m.entries[key]; ok {
		e.value = value
		return
	}

	e := &entry[K, V]{
		value: value,
	}
	e.node = m.order.PushBack(key)
	m.entries[key] = e
}

func (m *OrderedMap[K, V]) Delete(key K) {
	if _, ok := m.entries[key]; !ok {
		return
	}

	e := m.entries[key]
	m.order.Remove(e.node)
	delete(m.entries, key)
}

func (m *OrderedMap[K, V]) Len() int {
	return len(m.entries)
}

func (m *OrderedMap[K, V]) Clear() {
	m.order = xlist.New[K]()
	clear(m.entries)
}

func (m *OrderedMap[K, V]) Clone() *OrderedMap[K, V] {
	order := xlist.New[K]()
	entries := make(map[K]*entry[K, V])

	for k := range m.order.Values() {
		node := order.PushFront(k)
		entries[k] = &entry[K, V]{
			node:  node,
			value: m.Value(k),
		}
	}

	return &OrderedMap[K, V]{
		entries: entries,
		order:   order,
	}
}

func (m *OrderedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k := range m.order.Values() {
			v := m.entries[k].value
			if !yield(k, v) {
				return
			}
		}
	}
}

func (m *OrderedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range m.order.Values() {
			if !yield(k) {
				return
			}
		}
	}
}

func (m *OrderedMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for k := range m.order.Values() {
			v := m.entries[k].value
			if !yield(v) {
				return
			}
		}
	}
}
