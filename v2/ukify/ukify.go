// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ukify

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ironcore-dev/ironcore-image/v2/xio"
)

// Section names per the UKI spec (UAPI group)
const (
	SectionOSRel   = ".osrel"
	SectionCmdline = ".cmdline"
	SectionInitrd  = ".initrd"
	SectionLinux   = ".linux"
)

// PE/COFF constants
const (
	peSignature       = 0x00004550 // "PE\0\0"
	pe32PlusMagic     = 0x020b
	imageScnMemRead   = 0x40000000
	imageScnCntInitD  = 0x00000040
	sectionHeaderSize = 40
)

// BuildOptions configures UKI construction.
type BuildOptions struct {
	// Stub is the EFI stub binary.
	Stub xio.Source
	// Kernel is the linux kernel binary (required).
	Kernel xio.Source
	// Initrds are one or more initrd layers (concatenated in order).
	Initrds []xio.Source
	// Cmdline is the kernel command line.
	Cmdline string
	// OSRelease is the os-release content.
	OSRelease string
}

// Build constructs a UKI PE/COFF binary and writes it to w.
func Build(w io.Writer, opts BuildOptions) error {
	if opts.Stub == nil {
		return fmt.Errorf("stub is required")
	}
	if opts.Kernel == nil {
		return fmt.Errorf("kernel is required")
	}

	stubData, err := readAll(opts.Stub)
	if err != nil {
		return fmt.Errorf("reading stub: %w", err)
	}

	pe, err := parsePE(stubData)
	if err != nil {
		return fmt.Errorf("parsing stub: %w", err)
	}

	flags := uint32(imageScnMemRead | imageScnCntInitD)

	if opts.OSRelease != "" {
		pe.addBytesSection(SectionOSRel, []byte(opts.OSRelease+"\n"), flags)
	}
	if opts.Cmdline != "" {
		pe.addBytesSection(SectionCmdline, append([]byte(opts.Cmdline), 0), flags)
	}

	if len(opts.Initrds) > 0 {
		var total int64
		for i, rd := range opts.Initrds {
			sz, err := rd.Size()
			if err != nil {
				return fmt.Errorf("sizing initrd %d: %w", i, err)
			}
			total += sz
		}
		pe.addSourceSection(SectionInitrd, concatSources(opts.Initrds), total, flags)
	}

	// Kernel must be last
	kernelSize, err := opts.Kernel.Size()
	if err != nil {
		return fmt.Errorf("sizing kernel: %w", err)
	}
	pe.addSourceSection(SectionLinux, opts.Kernel, kernelSize, flags)

	return pe.marshal(w)
}

func readAll(s xio.Source) ([]byte, error) {
	rc, err := s.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// sourceSlice presents a sequence of sources as a single xio.Source by
// concatenating them on read. Size is the sum of underlying sizes.
type sourceSlice []xio.Source

func (s sourceSlice) Open() (io.ReadCloser, error) {
	readers := make([]io.Reader, 0, len(s))
	closers := make([]io.Closer, 0, len(s))
	for _, src := range s {
		rc, err := src.Open()
		if err != nil {
			for _, c := range closers {
				_ = c.Close()
			}
			return nil, err
		}
		readers = append(readers, rc)
		closers = append(closers, rc)
	}
	return &multiReadCloser{Reader: io.MultiReader(readers...), closers: closers}, nil
}

func (s sourceSlice) Size() (int64, error) {
	var total int64
	for _, src := range s {
		sz, err := src.Size()
		if err != nil {
			return 0, err
		}
		total += sz
	}
	return total, nil
}

func concatSources(srcs []xio.Source) xio.Source {
	if len(srcs) == 1 {
		return srcs[0]
	}
	return sourceSlice(srcs)
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiReadCloser) Close() error {
	var firstErr error
	for _, c := range m.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// peImage holds parsed PE state for manipulation.
type peImage struct {
	dosHeader        []byte // everything up to PE signature
	peOffset         uint32
	coffHeader       coffFileHeader
	optHeaderRaw     []byte
	optHeaderOffset  int // offset into raw where optional header starts
	sectionAlignment uint32
	fileAlignment    uint32
	sections         []peSection

	// Offset where section headers start in the file
	sectionTableOffset int
}

type coffFileHeader struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

// peSection describes a section in the output. Exactly one of data or source
// provides the section's bytes.
type peSection struct {
	header sectionHeader

	// data holds the section payload when it fits in memory (existing stub
	// sections plus small additions like .osrel and .cmdline).
	data []byte
	// source streams the section payload when it's too large to buffer
	// (.initrd and .linux). size is the authoritative byte count.
	source xio.Source
	size   uint32
}

func (s *peSection) payloadSize() uint32 {
	if s.source != nil {
		return s.size
	}
	return uint32(len(s.data))
}

func (s *peSection) writePayload(w io.Writer) error {
	if s.source != nil {
		rc, err := s.source.Open()
		if err != nil {
			return err
		}
		defer func() { _ = rc.Close() }()
		_, err = io.Copy(w, rc)
		return err
	}
	_, err := w.Write(s.data)
	return err
}

type sectionHeader struct {
	Name                 [8]byte
	VirtualSize          uint32
	VirtualAddress       uint32
	SizeOfRawData        uint32
	PointerToRawData     uint32
	PointerToRelocations uint32
	PointerToLinenumbers uint32
	NumberOfRelocations  uint16
	NumberOfLinenumbers  uint16
	Characteristics      uint32
}

func parsePE(data []byte) (*peImage, error) {
	if len(data) < 64 {
		return nil, fmt.Errorf("file too small")
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("not a PE file: missing MZ header")
	}

	peOffset := binary.LittleEndian.Uint32(data[60:64])
	if int(peOffset)+4 > len(data) {
		return nil, fmt.Errorf("invalid PE offset")
	}

	sig := binary.LittleEndian.Uint32(data[peOffset : peOffset+4])
	if sig != peSignature {
		return nil, fmt.Errorf("invalid PE signature")
	}

	img := &peImage{
		dosHeader: data[:peOffset],
		peOffset:  peOffset,
	}

	coffOffset := peOffset + 4
	r := bytes.NewReader(data[coffOffset:])
	if err := binary.Read(r, binary.LittleEndian, &img.coffHeader); err != nil {
		return nil, fmt.Errorf("reading COFF header: %w", err)
	}

	optOffset := int(coffOffset) + 20 // COFF header is 20 bytes
	optSize := int(img.coffHeader.SizeOfOptionalHeader)
	img.optHeaderOffset = optOffset
	img.optHeaderRaw = data[optOffset : optOffset+optSize]

	// Read alignment values from optional header
	// PE32+: SectionAlignment at offset 32, FileAlignment at offset 36
	optMagic := binary.LittleEndian.Uint16(img.optHeaderRaw[0:2])
	if optMagic != pe32PlusMagic {
		return nil, fmt.Errorf("only PE32+ is supported, got magic %#x", optMagic)
	}
	img.sectionAlignment = binary.LittleEndian.Uint32(img.optHeaderRaw[32:36])
	img.fileAlignment = binary.LittleEndian.Uint32(img.optHeaderRaw[36:40])

	// Parse existing sections
	img.sectionTableOffset = optOffset + optSize
	for i := 0; i < int(img.coffHeader.NumberOfSections); i++ {
		off := img.sectionTableOffset + i*sectionHeaderSize
		var hdr sectionHeader
		hr := bytes.NewReader(data[off : off+sectionHeaderSize])
		if err := binary.Read(hr, binary.LittleEndian, &hdr); err != nil {
			return nil, fmt.Errorf("reading section header %d: %w", i, err)
		}

		var sdata []byte
		if hdr.SizeOfRawData > 0 && int(hdr.PointerToRawData+hdr.SizeOfRawData) <= len(data) {
			sdata = data[hdr.PointerToRawData : hdr.PointerToRawData+hdr.SizeOfRawData]
		}
		img.sections = append(img.sections, peSection{header: hdr, data: sdata})
	}

	return img, nil
}

func (img *peImage) addBytesSection(name string, data []byte, characteristics uint32) {
	hdr := img.newSectionHeader(name, uint32(len(data)), characteristics)
	img.sections = append(img.sections, peSection{header: hdr, data: data})
}

func (img *peImage) addSourceSection(name string, src xio.Source, size int64, characteristics uint32) {
	hdr := img.newSectionHeader(name, uint32(size), characteristics)
	img.sections = append(img.sections, peSection{header: hdr, source: src, size: uint32(size)})
}

func (img *peImage) newSectionHeader(name string, payloadSize, characteristics uint32) sectionHeader {
	var nameBytes [8]byte
	copy(nameBytes[:], name)

	// Compute virtual address: after last existing section
	var nextVA uint32
	if len(img.sections) > 0 {
		last := img.sections[len(img.sections)-1].header
		nextVA = align(last.VirtualAddress+last.VirtualSize, img.sectionAlignment)
	}

	// Compute raw offset: after last existing section's raw data
	var nextRaw uint32
	if len(img.sections) > 0 {
		last := img.sections[len(img.sections)-1].header
		nextRaw = align(last.PointerToRawData+last.SizeOfRawData, img.fileAlignment)
	}

	return sectionHeader{
		Name:             nameBytes,
		VirtualSize:      payloadSize,
		VirtualAddress:   nextVA,
		SizeOfRawData:    alignUp(payloadSize, img.fileAlignment),
		PointerToRawData: nextRaw,
		Characteristics:  characteristics,
	}
}

func (img *peImage) marshal(w io.Writer) error {
	// We need to ensure the section table fits in the headers.
	// Compute new header size
	newNumSections := len(img.sections)
	sectionTableEnd := img.sectionTableOffset + newNumSections*sectionHeaderSize
	headersSize := alignUp(uint32(sectionTableEnd), img.fileAlignment)

	// Rebase all sections' raw offsets relative to new headers size
	offset := headersSize
	for i := range img.sections {
		img.sections[i].header.PointerToRawData = offset
		img.sections[i].header.SizeOfRawData = alignUp(img.sections[i].payloadSize(), img.fileAlignment)
		offset += img.sections[i].header.SizeOfRawData
	}

	// Compute SizeOfImage (virtual extent of last section, aligned)
	lastSec := img.sections[len(img.sections)-1].header
	sizeOfImage := align(lastSec.VirtualAddress+lastSec.VirtualSize, img.sectionAlignment)

	// Build the headers in a buffer first so we can pad them to headersSize
	// before streaming section payloads.
	var hdrBuf bytes.Buffer

	// Write DOS header
	hdrBuf.Write(img.dosHeader)

	// Write PE signature
	if err := binary.Write(&hdrBuf, binary.LittleEndian, uint32(peSignature)); err != nil {
		return err
	}

	// Write COFF header with updated section count
	coff := img.coffHeader
	coff.NumberOfSections = uint16(newNumSections)
	if err := binary.Write(&hdrBuf, binary.LittleEndian, &coff); err != nil {
		return err
	}

	// Write optional header with updated SizeOfImage and SizeOfHeaders
	optCopy := make([]byte, len(img.optHeaderRaw))
	copy(optCopy, img.optHeaderRaw)
	// SizeOfImage is at offset 56 in PE32+ optional header
	binary.LittleEndian.PutUint32(optCopy[56:60], sizeOfImage)
	// SizeOfHeaders is at offset 60 in PE32+ optional header
	binary.LittleEndian.PutUint32(optCopy[60:64], headersSize)
	// Zero out checksum (offset 64) — not needed unless signing
	binary.LittleEndian.PutUint32(optCopy[64:68], 0)
	hdrBuf.Write(optCopy)

	// Write section headers
	for _, sec := range img.sections {
		if err := binary.Write(&hdrBuf, binary.LittleEndian, &sec.header); err != nil {
			return err
		}
	}

	// Pad headers to headersSize
	if pad := int(headersSize) - hdrBuf.Len(); pad > 0 {
		hdrBuf.Write(make([]byte, pad))
	}

	if _, err := w.Write(hdrBuf.Bytes()); err != nil {
		return err
	}

	// Write section data (padded to FileAlignment)
	for i := range img.sections {
		sec := &img.sections[i]
		if err := sec.writePayload(w); err != nil {
			return fmt.Errorf("writing section %s: %w", sectionName(sec.header.Name), err)
		}
		if pad := int(sec.header.SizeOfRawData) - int(sec.payloadSize()); pad > 0 {
			if _, err := w.Write(make([]byte, pad)); err != nil {
				return err
			}
		}
	}

	return nil
}

func sectionName(name [8]byte) string {
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return string(name[:n])
}

func align(value, alignment uint32) uint32 {
	return alignUp(value, alignment)
}

func alignUp(value, alignment uint32) uint32 {
	if alignment == 0 {
		return value
	}
	return (value + alignment - 1) &^ (alignment - 1)
}
