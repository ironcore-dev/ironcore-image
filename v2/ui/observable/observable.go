// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package observable

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/ironcore-dev/ironcore-image/v2/xio"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
)

type Listeners[Listener any] struct {
	mu sync.RWMutex

	items map[*Listener]struct{}
}

func (l *Listeners[Listener]) Add(listener Listener) (remove func()) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.items == nil {
		l.items = make(map[*Listener]struct{})
	}

	ptr := &listener
	l.items[ptr] = struct{}{}
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		delete(l.items, ptr)
	}
}

func (l *Listeners[Listener]) snapshot() []Listener {
	l.mu.RLock()
	defer l.mu.RUnlock()

	items := make([]Listener, 0, len(l.items))
	for ptr := range l.items {
		items = append(items, *ptr)
	}
	return items
}

func (l *Listeners[Listener]) Emit(f func(Listener)) {
	for _, l := range l.snapshot() {
		f(l)
	}
}

type PushEvent interface {
	GetDescriptor() ocispec.Descriptor
}

type PushStart struct {
	Descriptor ocispec.Descriptor
}

func (p PushStart) GetDescriptor() ocispec.Descriptor {
	return p.Descriptor
}

type PushProgress struct {
	Descriptor ocispec.Descriptor
	BytesRead  int64
}

func (p PushProgress) GetDescriptor() ocispec.Descriptor {
	return p.Descriptor
}

type PushDone struct {
	Descriptor ocispec.Descriptor
	Error      error
}

func (p PushDone) GetDescriptor() ocispec.Descriptor {
	return p.Descriptor
}

type PushListener interface {
	OnPush(event PushEvent)
}

type PushListenerFunc func(event PushEvent)

func (f PushListenerFunc) OnPush(event PushEvent) {
	f(event)
}

type pusher struct {
	listeners Listeners[PushListener]
}

func (op *pusher) AddListener(l PushListener) func() {
	return op.listeners.Add(l)
}

type Pusher interface {
	content.Pusher
	AddListener(l PushListener) func()
}

func (op *pusher) push(
	ctx context.Context,
	ps content.Pusher,
	expected ocispec.Descriptor,
	content io.Reader,
) error {
	op.listeners.Emit(func(l PushListener) { l.OnPush(PushStart{Descriptor: expected}) })

	var (
		total    int64
		lastEmit time.Time
	)
	err := ps.Push(ctx, expected, xio.ReaderFunc(func(p []byte) (n int, err error) {
		n, err = content.Read(p)
		total += int64(n)
		if time.Since(lastEmit) >= 50*time.Millisecond || errors.Is(err, io.EOF) {
			lastEmit = time.Now()
			op.listeners.Emit(func(l PushListener) { l.OnPush(PushProgress{Descriptor: expected, BytesRead: total}) })
		}
		return n, err
	}))

	op.listeners.Emit(func(l PushListener) { l.OnPush(PushDone{Descriptor: expected, Error: err}) })

	return err
}

type OCIStore struct {
	*oci.Store
	pusher
}

func (s *OCIStore) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	return s.push(ctx, s.Store, expected, content)
}

type RemoteRepository struct {
	*remote.Repository
	pusher
}

func (r *RemoteRepository) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	return r.push(ctx, r.Repository, expected, content)
}
