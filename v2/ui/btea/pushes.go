// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package btea

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ironcore-dev/ironcore-image/v2/image/uki"
	"github.com/ironcore-dev/ironcore-image/v2/ui/observable"
	"github.com/ironcore-dev/ironcore-image/v2/xcontainer/orderedmap"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type pushes struct {
	titleFunc func(ocispec.Descriptor) string
	verb      string
	items     *orderedmap.OrderedMap[digest.Digest, *push]
}

func newPushes(titleFunc func(ocispec.Descriptor) string, verb string) pushes {
	return pushes{
		titleFunc: titleFunc,
		verb:      verb,
		items:     orderedmap.New[digest.Digest, *push](),
	}
}

func (ps pushes) title(desc ocispec.Descriptor) string {
	if ps.titleFunc != nil {
		return ps.titleFunc(desc)
	}
	return desc.MediaType
}

type PushItemFrame struct {
	desc ocispec.Descriptor
	msg  tea.Msg
}

func wrapPushItemMsg(desc ocispec.Descriptor, itemCmd tea.Cmd) tea.Cmd {
	if itemCmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := itemCmd()
		return PushItemFrame{
			desc: desc,
			msg:  msg,
		}
	}
}

func (ps pushes) Len() int {
	return ps.items.Len()
}

func (ps pushes) Update(msg tea.Msg) (pushes, tea.Cmd) {
	switch msg := msg.(type) {
	case observable.PushStart:
		p := newPush(ps.title(msg.Descriptor), ps.verb, msg.Descriptor)
		ps.items.Set(msg.Descriptor.Digest, p)
		return ps, wrapPushItemMsg(msg.Descriptor, p.Tick)
	case observable.PushDone:
		item, ok := ps.items.Get(msg.Descriptor.Digest)
		if !ok {
			return ps, nil
		}

		ps.items.Delete(msg.Descriptor.Digest)
		return ps, item.Result(msg.Error)
	case observable.PushProgress:
		item, ok := ps.items.Get(msg.Descriptor.Digest)
		if !ok {
			return ps, nil
		}

		return ps, wrapPushItemMsg(msg.Descriptor, item.Progress(msg.BytesRead))
	case PushItemFrame:
		item, ok := ps.items.Get(msg.desc.Digest)
		if !ok {
			return ps, nil
		}

		return ps, wrapPushItemMsg(msg.desc, item.Update(msg.msg))
	}
	return ps, nil
}

func (ps pushes) View() string {
	var (
		sb      strings.Builder
		needSep bool
	)
	for pb := range ps.items.Values() {
		if needSep {
			sb.WriteString("\n")
		}
		needSep = true
		sb.WriteString(pb.View())
	}
	return sb.String()
}

func DefaultTitleFunc(desc ocispec.Descriptor) string {
	switch desc.MediaType {
	case uki.MediaTypeLayerKernel:
		return "kernel"
	case uki.MediaTypeLayerInitrd, uki.MediaTypeLayerInitrdGzip, uki.MediaTypeLayerInitrdXz,
		uki.MediaTypeLayerInitrdLz4, uki.MediaTypeLayerInitrdZstd:
		return "initrd"
	case uki.MediaTypeLayerStub:
		return "stub"
	case uki.MediaTypeConfig:
		return "config"
	case ocispec.MediaTypeImageManifest:
		return "manifest"
	default:
		return desc.MediaType
	}
}
