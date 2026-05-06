// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xio

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
)

type Adder interface {
	Add(n int64)
}

type AdderFunc func(n int64)

func (f AdderFunc) Add(n int64) { f(n) }

type teeByteCountReader struct {
	rd    io.Reader
	adder Adder
}

func (r *teeByteCountReader) Read(p []byte) (int, error) {
	n, err := r.rd.Read(p)
	r.adder.Add(int64(n))
	return n, err
}

func TeeByteCountReader(rd io.Reader, adder Adder) io.Reader {
	return &teeByteCountReader{rd, adder}
}

func TeeByteCountInt64Reader(rd io.Reader, i *int64) io.Reader {
	return TeeByteCountReader(rd, AdderFunc(func(n int64) {
		*i = *i + n
	}))
}

type ReaderFunc func(p []byte) (n int, err error)

func (f ReaderFunc) Read(p []byte) (n int, err error) { return f(p) }

type Source interface {
	Open() (io.ReadCloser, error)
	Size() (int64, error)
}

type FileSource string

func (s FileSource) Open() (io.ReadCloser, error) {
	return os.Open(string(s))
}

func (s FileSource) Size() (int64, error) {
	stat, err := os.Stat(string(s))
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", s, err)
	}
	return stat.Size(), nil
}

type fsFileSource struct {
	fs   fs.FS
	name string
}

func (fs *fsFileSource) Open() (io.ReadCloser, error) {
	return fs.fs.Open(fs.name)
}

func (fs *fsFileSource) Size() (int64, error) {
	f, err := fs.fs.Open(fs.name)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", fs.name, err)
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", fs.name, err)
	}
	return stat.Size(), nil
}

func FSFileSource(fs fs.FS, name string) Source {
	return &fsFileSource{fs, name}
}

type BytesSource []byte

func (s BytesSource) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s)), nil
}

func (s BytesSource) Size() (int64, error) {
	return int64(len(s)), nil
}
