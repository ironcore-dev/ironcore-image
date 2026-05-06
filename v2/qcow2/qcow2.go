// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package qcow2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	qcow2rd "github.com/lima-vm/go-qcow2reader/image/qcow2"
)

// qcow2 header layout (big-endian):
//
//	 0..3    magic ("QFI\xfb")
//	 4..7    version (2 or 3)
//	 8..15   backing_file_offset
//	16..19   backing_file_size
//	v2: header is fixed at 72 bytes; extensions follow at offset 72.
//	v3: 100..103 holds header_length; extensions follow at that offset.
const (
	extMagicEnd           = 0x00000000
	extMagicBackingFormat = 0xE2792ACA
)

var qcow2Magic = [4]byte{'Q', 'F', 'I', 0xfb}

type qcow2 struct{}

func Qcow2() Interface { return qcow2{} }

func (qcow qcow2) UnsafeRemoveBacking(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var hdr [104]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	if [4]byte(hdr[0:4]) != qcow2Magic {
		return fmt.Errorf("not a qcow2 image: %s", path)
	}
	version := binary.BigEndian.Uint32(hdr[4:8])
	if version != 2 && version != 3 {
		return fmt.Errorf("unsupported qcow2 version: %d", version)
	}

	extStart := int64(72)
	if version >= 3 {
		extStart = int64(binary.BigEndian.Uint32(hdr[100:104]))
	}
	if err := qcow.dropBackingFormatExt(f, extStart); err != nil {
		return fmt.Errorf("drop backing-format extension: %w", err)
	}

	var zero [12]byte // backing_file_offset (8) + backing_file_size (4)
	if _, err := f.WriteAt(zero[:], 8); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	return f.Sync()
}

// dropBackingFormatExt rewrites the extension list starting at extStart,
// omitting any QCOW2_EXT_MAGIC_BACKING_FORMAT record. Other extensions are
// preserved verbatim. The terminator extension is always present.
func (qcow2) dropBackingFormatExt(f *os.File, extStart int64) error {
	var kept []byte
	off := extStart
	for {
		var eh [8]byte
		if _, err := f.ReadAt(eh[:], off); err != nil {
			return err
		}
		magic := binary.BigEndian.Uint32(eh[0:4])
		dataLen := binary.BigEndian.Uint32(eh[4:8])
		padded := (dataLen + 7) &^ 7
		recLen := int64(8) + int64(padded)

		if magic == extMagicEnd {
			break
		}
		if magic != extMagicBackingFormat {
			rec := make([]byte, recLen)
			if _, err := f.ReadAt(rec, off); err != nil {
				return err
			}
			kept = append(kept, rec...)
		}
		off += recLen
	}

	// Append terminator and write back from extStart, padding the original
	// extension span with zeros so leftover bytes from a removed record
	// don't get parsed as garbage.
	var term [8]byte // magic=0, len=0
	kept = append(kept, term[:]...)

	origLen := off + 8 - extStart
	if int64(len(kept)) < origLen {
		kept = append(kept, make([]byte, origLen-int64(len(kept)))...)
	}
	_, err := f.WriteAt(kept, extStart)
	return err
}

type qcowChain struct {
	layers []*qcow2rd.Qcow2
	size   int64
}

func openChain(chain []string) (*qcowChain, error) {
	res := &qcowChain{}
	for _, c := range chain {
		f, err := os.Open(c)
		if err != nil {
			return nil, err
		}

		img, err := qcow2rd.Open(f, nil)
		if err != nil {
			_ = res.Close()
			return nil, fmt.Errorf("[%s] open image: %w", c, err)
		}

		res.layers = append(res.layers, img)
		res.size = max(res.size, img.Size())
	}
	return res, nil
}

func (c *qcowChain) Close() error {
	var errs []error
	for _, layer := range c.layers {
		if err := layer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// readAt resolves a single byte range by walking the chain top-down.
// For each layer, ask Extent: if it has data, read from there; otherwise
// fall through to the next layer. If nothing in the chain has data,
// the range is zero.
func (c *qcowChain) readAt(p []byte, off int64) error {
	for i, layer := range c.layers {
		ext, err := layer.Extent(off, int64(len(p)))
		if err != nil {
			return err
		}
		// Clamp to what this extent covers from `off`.
		extEnd := ext.Start + ext.Length
		n := extEnd - off
		if n > int64(len(p)) {
			n = int64(len(p))
		}

		switch {
		case ext.Zero:
			// Explicit zero cluster — authoritative, stop here.
			zero(p[:n])
		case !ext.Allocated:
			// Hole: this layer has nothing here. Try next layer.
			if i == len(c.layers)-1 {
				zero(p[:n]) // bottom of chain, range is zero
			} else {
				if err := c.readAtLayer(p[:n], off, i+1); err != nil {
					return err
				}
			}
		default:
			if _, err := layer.ReadAt(p[:n], off); err != nil {
				return err
			}
		}

		// Advance within p for any remainder of the requested range.
		if n == int64(len(p)) {
			return nil
		}
		p = p[n:]
		off += n
	}
	return nil
}

func (c *qcowChain) readAtLayer(p []byte, off int64, start int) error {
	sub := &qcowChain{layers: c.layers[start:], size: c.size}
	return sub.readAt(p, off)
}

func zero(p []byte) {
	for i := range p {
		p[i] = 0
	}
}

func (qcow qcow2) WriteChain(chain []string, filename string) error {
	c, err := openChain(chain)
	if err != nil {
		return fmt.Errorf("open chain: %w", err)
	}
	defer func() { _ = c.Close() }()

	dst, err := os.OpenFile(filename, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	const chunk = 2 << 20
	buf := make([]byte, chunk)
	for off := int64(0); off < c.size; off += int64(chunk) {
		n := int64(chunk)
		if off+n > c.size {
			n = c.size - off
		}

		if err := c.readAt(buf, off); err != nil {
			return fmt.Errorf("read at offset %d: %w", off, err)
		}

		if _, err := dst.Write(buf[:n]); err != nil {
			return fmt.Errorf("write data (offset %d): %w", off, err)
		}
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}
