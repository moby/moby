package btf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
)

// stringTable contains a sequence of null-terminated strings.
//
// It is safe for concurrent use.
type stringTable struct {
	base  *stringTable
	bytes []byte

	mu    sync.Mutex
	cache map[uint32]string
}

// sizedReader is implemented by bytes.Reader, io.SectionReader, strings.Reader, etc.
type sizedReader interface {
	io.Reader
	Size() int64
}

func readStringTable(r sizedReader, base *stringTable) (*stringTable, error) {
	bytes := make([]byte, r.Size())
	if _, err := io.ReadFull(r, bytes); err != nil {
		return nil, err
	}

	return newStringTable(bytes, base)
}

func newStringTable(bytes []byte, base *stringTable) (*stringTable, error) {
	// When parsing split BTF's string table, the first entry offset is derived
	// from the last entry offset of the base BTF.
	firstStringOffset := uint32(0)
	if base != nil {
		firstStringOffset = uint32(len(base.bytes))
	}

	if len(bytes) > 0 {
		if bytes[len(bytes)-1] != 0 {
			return nil, errors.New("string table isn't null terminated")
		}

		if firstStringOffset == 0 && bytes[0] != 0 {
			return nil, errors.New("first item in string table is non-empty")
		}
	}

	return &stringTable{base: base, bytes: bytes}, nil
}

func (st *stringTable) Lookup(offset uint32) (string, error) {
	// Fast path: zero offset is the empty string, looked up frequently.
	if offset == 0 {
		return "", nil
	}

	b, err := st.lookupSlow(offset)
	return string(b), err
}

func (st *stringTable) LookupBytes(offset uint32) ([]byte, error) {
	// Fast path: zero offset is the empty string, looked up frequently.
	if offset == 0 {
		return nil, nil
	}

	return st.lookupSlow(offset)
}

func (st *stringTable) lookupSlow(offset uint32) ([]byte, error) {
	if st.base != nil {
		n := uint32(len(st.base.bytes))
		if offset < n {
			return st.base.lookupSlow(offset)
		}
		offset -= n
	}

	if offset > uint32(len(st.bytes)) {
		return nil, fmt.Errorf("offset %d is out of bounds of string table", offset)
	}

	if offset > 0 && st.bytes[offset-1] != 0 {
		return nil, fmt.Errorf("offset %d is not the beginning of a string", offset)
	}

	i := bytes.IndexByte(st.bytes[offset:], 0)
	if i == -1 {
		return nil, fmt.Errorf("string at offset %d is not null terminated", offset)
	}
	return st.bytes[offset : offset+uint32(i)], nil
}

// LookupCache returns the string at the given offset, caching the result
// for future lookups.
func (cst *stringTable) LookupCached(offset uint32) (string, error) {
	// Fast path: zero offset is the empty string, looked up frequently.
	if offset == 0 {
		return "", nil
	}

	cst.mu.Lock()
	defer cst.mu.Unlock()

	if str, ok := cst.cache[offset]; ok {
		return str, nil
	}

	str, err := cst.Lookup(offset)
	if err != nil {
		return "", err
	}

	if cst.cache == nil {
		cst.cache = make(map[uint32]string)
	}
	cst.cache[offset] = str
	return str, nil
}

// stringTableBuilder builds BTF string tables.
type stringTableBuilder struct {
	length  uint32
	strings map[string]uint32
}

// newStringTableBuilder creates a builder with the given capacity.
//
// capacity may be zero.
func newStringTableBuilder(capacity int) *stringTableBuilder {
	var stb stringTableBuilder

	if capacity == 0 {
		// Use the runtime's small default size.
		stb.strings = make(map[string]uint32)
	} else {
		stb.strings = make(map[string]uint32, capacity)
	}

	// Ensure that the empty string is at index 0.
	stb.append("")
	return &stb
}

// Add a string to the table.
//
// Adding the same string multiple times will only store it once.
func (stb *stringTableBuilder) Add(str string) (uint32, error) {
	if strings.IndexByte(str, 0) != -1 {
		return 0, fmt.Errorf("string contains null: %q", str)
	}

	offset, ok := stb.strings[str]
	if ok {
		return offset, nil
	}

	return stb.append(str), nil
}

func (stb *stringTableBuilder) append(str string) uint32 {
	offset := stb.length
	stb.length += uint32(len(str)) + 1
	stb.strings[str] = offset
	return offset
}

// Lookup finds the offset of a string in the table.
//
// Returns an error if str hasn't been added yet.
func (stb *stringTableBuilder) Lookup(str string) (uint32, error) {
	offset, ok := stb.strings[str]
	if !ok {
		return 0, fmt.Errorf("string %q is not in table", str)
	}

	return offset, nil
}

// Length returns the length in bytes.
func (stb *stringTableBuilder) Length() int {
	return int(stb.length)
}

// AppendEncoded appends the string table to the end of the provided buffer.
func (stb *stringTableBuilder) AppendEncoded(buf []byte) []byte {
	n := len(buf)
	buf = append(buf, make([]byte, stb.Length())...)
	strings := buf[n:]
	for str, offset := range stb.strings {
		copy(strings[offset:], str)
	}
	return buf
}

// Copy the string table builder.
func (stb *stringTableBuilder) Copy() *stringTableBuilder {
	return &stringTableBuilder{
		stb.length,
		maps.Clone(stb.strings),
	}
}
