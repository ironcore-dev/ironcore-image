// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/spf13/pflag"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

type Provider interface {
	Store(ctx context.Context) (*oci.Store, error)
}

var DefaultOptions Options

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		DefaultOptions.StorageDir = filepath.Join(home, ".ironcore-image")
	}
}

type Options struct {
	StorageDir string
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.StorageDir, "storage-dir", o.StorageDir, "Directory to create local OCI storage layout at.")
}

func (o *Options) Store(ctx context.Context) (*oci.Store, error) {
	if o.StorageDir == "" {
		return nil, fmt.Errorf("--storage-dir is required")
	}

	storage, err := oci.NewWithContext(ctx, o.StorageDir)
	if err != nil {
		return nil, fmt.Errorf("creating oci storage: %w", err)
	}

	return storage, nil
}

type Platforms struct {
	Platforms       []string
	PlatformsByKind map[string][]string
}

func (o *Platforms) CombinedFor(kind string) []string {
	res := make([]string, len(o.Platforms)+len(o.PlatformsByKind))
	copy(res, o.Platforms)
	copy(res[len(o.Platforms):], o.PlatformsByKind[kind])
	return res
}

type platformsVar struct {
	changed bool
	value   *Platforms
}

func newPlatformsVar(val Platforms, p *Platforms) *platformsVar {
	ssv := new(platformsVar)
	ssv.value = p
	*ssv.value = val
	return ssv
}

func (p *platformsVar) Type() string {
	return "platforms"
}

func (p *platformsVar) Set(val string) error {
	var ss []string
	n := strings.Count(val, ":")
	switch n {
	case 0, 1:
		ss = append(ss, strings.Trim(val, `"`))
	default:
		r := csv.NewReader(strings.NewReader(val))
		var err error
		ss, err = r.Read()
		if err != nil {
			return err
		}
	}

	out := Platforms{}
	for _, pair := range ss {
		parts := strings.SplitN(pair, "=", 2)
		switch len(parts) {
		case 1:
			out.Platforms = append(out.Platforms, parts[0])
		case 2:
			kind := parts[0]
			platform := parts[1]
			if out.PlatformsByKind == nil {
				out.PlatformsByKind = make(map[string][]string)
			}
			out.PlatformsByKind[kind] = append(out.PlatformsByKind[kind], platform)
		default:
			return fmt.Errorf("invalid format for platform: %s", pair)
		}
	}
	if !p.changed {
		*p.value = out
	} else {
		p.value.Platforms = append(p.value.Platforms, out.Platforms...)
		maps.Copy(p.value.PlatformsByKind, out.PlatformsByKind)
	}
	p.changed = true
	return nil
}

func (p *platformsVar) String() string {
	keys := make([]string, 0, len(p.value.PlatformsByKind))
	for k := range p.value.PlatformsByKind {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	records := make([]string, 0, len(p.value.Platforms)+len(p.value.PlatformsByKind))
	records = append(records, p.value.Platforms...)
	for _, k := range keys {
		v := p.value.PlatformsByKind[k]
		records = append(records, fmt.Sprintf("%s:%s", k, v))
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(records); err != nil {
		panic(err)
	}
	w.Flush()
	return "[" + strings.TrimSpace(buf.String()) + "]"
}

func PlatformsVar(fs *pflag.FlagSet, p *Platforms, name string, value Platforms, usage string) {
	fs.Var(newPlatformsVar(value, p), name, usage)
}

func ShortEncoded(d digest.Digest) string {
	enc := d.Encoded()
	return enc[:min(12, len(enc))]
}

func RepositoryFor(ref registry.Reference) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref.Registry + "/" + ref.Repository)
	if err != nil {
		return nil, err
	}

	repo.PlainHTTP = true
	return repo, nil
}

func ParseReferenceDefaultLatest(artifact string) (registry.Reference, error) {
	ref, err := registry.ParseReference(artifact)
	if err != nil {
		return ref, err
	}

	ref.Reference = ref.ReferenceOrDefault()
	return ref, nil
}
