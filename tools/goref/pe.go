package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"unicode/utf16"

	"github.com/jgabor/spela/internal/dll"
)

// peStubNote is written into the DOS stub area of every PE stub so that no
// one mistakes a fixture for a real DLL.
const peStubNote = "spela fixture PE stub, not a real DLL"

// peStub builds a small PE32+ DLL image with one .rsrc section that holds a
// real resource tree with an RT_VERSION entry. Its VS_FIXEDFILEINFO carries
// the given file version. The image has no code; it only exists so version
// readers see the version.
func peStub(major, minor, build, revision uint16) []byte {
	const (
		peOffset        = 0x80
		fileAlignment   = 0x200
		sectionRVA      = 0x1000
		sectionAlign    = 0x1000
		optionalSize    = 240
		resourceDirSize = 16
		entrySize       = 8
	)

	version := versionResource(major, minor, build, revision)

	// Resource tree: root (type) -> name -> language -> data entry -> blob.
	rsrc := make([]byte, 0, 256)
	appendDirectory := func(id uint32, offset uint32, subdirectory bool) {
		header := make([]byte, resourceDirSize)
		binary.LittleEndian.PutUint16(header[14:], 1) // one id entry
		rsrc = append(rsrc, header...)
		entry := make([]byte, entrySize)
		binary.LittleEndian.PutUint32(entry[0:], id)
		if subdirectory {
			offset |= 0x80000000
		}
		binary.LittleEndian.PutUint32(entry[4:], offset)
		rsrc = append(rsrc, entry...)
	}
	appendDirectory(16, 24, true)     // RT_VERSION, next directory at 24
	appendDirectory(1, 48, true)      // name id 1, next directory at 48
	appendDirectory(0x409, 72, false) // en-US, data entry at 72
	dataEntry := make([]byte, 16)
	blobOffset := uint32(88)
	binary.LittleEndian.PutUint32(dataEntry[0:], sectionRVA+blobOffset)
	binary.LittleEndian.PutUint32(dataEntry[4:], uint32(len(version)))
	rsrc = append(rsrc, dataEntry...)
	rsrc = append(rsrc, version...)

	rawSize := (len(rsrc) + fileAlignment - 1) / fileAlignment * fileAlignment
	image := make([]byte, fileAlignment+rawSize)

	// DOS header.
	copy(image[0:], "MZ")
	binary.LittleEndian.PutUint32(image[0x3c:], peOffset)
	copy(image[0x40:], peStubNote)

	// PE signature and COFF header.
	offset := peOffset
	copy(image[offset:], "PE\x00\x00")
	offset += 4
	binary.LittleEndian.PutUint16(image[offset:], 0x8664) // AMD64
	binary.LittleEndian.PutUint16(image[offset+2:], 1)    // one section
	binary.LittleEndian.PutUint16(image[offset+16:], optionalSize)
	binary.LittleEndian.PutUint16(image[offset+18:], 0x2022) // executable, large address aware, DLL
	offset += 20

	// PE32+ optional header.
	optional := image[offset : offset+optionalSize]
	binary.LittleEndian.PutUint16(optional[0:], 0x20b)
	binary.LittleEndian.PutUint64(optional[24:], 0x180000000) // ImageBase
	binary.LittleEndian.PutUint32(optional[32:], sectionAlign)
	binary.LittleEndian.PutUint32(optional[36:], fileAlignment)
	binary.LittleEndian.PutUint16(optional[40:], 6) // OS major
	binary.LittleEndian.PutUint16(optional[48:], 6) // subsystem major
	binary.LittleEndian.PutUint32(optional[56:], sectionRVA+sectionAlign)
	binary.LittleEndian.PutUint32(optional[60:], fileAlignment)
	binary.LittleEndian.PutUint16(optional[68:], 2) // Windows GUI
	binary.LittleEndian.PutUint32(optional[108:], 16)
	// Data directory 2 is the resource table.
	binary.LittleEndian.PutUint32(optional[112+2*8:], sectionRVA)
	binary.LittleEndian.PutUint32(optional[112+2*8+4:], uint32(len(rsrc)))
	offset += optionalSize

	// Section header.
	section := image[offset : offset+40]
	copy(section[0:], ".rsrc")
	binary.LittleEndian.PutUint32(section[8:], uint32(len(rsrc)))
	binary.LittleEndian.PutUint32(section[12:], sectionRVA)
	binary.LittleEndian.PutUint32(section[16:], uint32(rawSize))
	binary.LittleEndian.PutUint32(section[20:], fileAlignment)
	binary.LittleEndian.PutUint32(section[36:], 0x40000040) // initialized data, readable

	copy(image[fileAlignment:], rsrc)
	return image
}

// versionResource builds a VS_VERSIONINFO block with only VS_FIXEDFILEINFO.
func versionResource(major, minor, build, revision uint16) []byte {
	key := utf16.Encode([]rune("VS_VERSION_INFO\x00"))
	block := make([]byte, 6, 92)
	for _, unit := range key {
		block = binary.LittleEndian.AppendUint16(block, unit)
	}
	for len(block)%4 != 0 {
		block = append(block, 0)
	}
	fileMS := uint32(major)<<16 | uint32(minor)
	fileLS := uint32(build)<<16 | uint32(revision)
	for _, value := range []uint32{
		0xFEEF04BD, 0x00010000, // signature, structure version
		fileMS, fileLS, // file version
		fileMS, fileLS, // product version
		0x3F, 0, // flags mask, flags
		0x40004, 2, 0, // NT Windows32, DLL, no subtype
		0, 0, // date
	} {
		block = binary.LittleEndian.AppendUint32(block, value)
	}
	binary.LittleEndian.PutUint16(block[0:], uint16(len(block)))
	binary.LittleEndian.PutUint16(block[2:], 52)
	return block
}

// peStubVersion makes a stub for a dotted version with up to four parts.
func peStubVersion(text string) []byte {
	var parts [4]uint16
	var index int
	for _, part := range splitDots(text) {
		var value uint16
		_, _ = fmt.Sscanf(part, "%d", &value)
		parts[index] = value
		index++
	}
	return peStub(parts[0], parts[1], parts[2], parts[3])
}

func splitDots(text string) []string {
	var parts []string
	start := 0
	for index := 0; index <= len(text); index++ {
		if index == len(text) || text[index] == '.' {
			parts = append(parts, text[start:index])
			start = index + 1
		}
	}
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return parts
}

// runPEVersion prints the version Go's DLL scanner reads from each file,
// next to the raw four VS_FIXEDFILEINFO file version parts. It only reads
// the files.
func runPEVersion(paths []string) error {
	for _, path := range paths {
		version, err := dll.GetDLLVersion(path)
		if err != nil {
			fmt.Printf("%s\terror: %v\n", path, err)
			continue
		}
		if version == "" {
			version = "(none)"
		}
		fmt.Printf("%s\t%s\t%s\n", path, version, rawFileVersion(path))
	}
	return nil
}

// rawFileVersion reads the four file version parts the same way Go's
// scanner finds them, without formatting.
func rawFileVersion(path string) string {
	file, err := pe.Open(path)
	if err != nil {
		return "raw: " + err.Error()
	}
	defer func() { _ = file.Close() }()
	section := file.Section(".rsrc")
	if section == nil {
		return "raw: no .rsrc"
	}
	data, err := section.Data()
	if err != nil {
		return "raw: " + err.Error()
	}
	key := utf16.Encode([]rune("VS_VERSION_INFO"))
	needle := make([]byte, 0, len(key)*2)
	for _, unit := range key {
		needle = binary.LittleEndian.AppendUint16(needle, unit)
	}
	start := bytes.Index(data, needle)
	if start < 0 {
		return "raw: no VS_VERSION_INFO"
	}
	signature := bytes.Index(data[start:], []byte{0xBD, 0x04, 0xEF, 0xFE})
	if signature < 0 || start+signature+16 > len(data) {
		return "raw: no VS_FIXEDFILEINFO"
	}
	offset := start + signature + 8
	ms := binary.LittleEndian.Uint32(data[offset:])
	ls := binary.LittleEndian.Uint32(data[offset+4:])
	return fmt.Sprintf("raw %d.%d.%d.%d", ms>>16, ms&0xffff, ls>>16, ls&0xffff)
}
