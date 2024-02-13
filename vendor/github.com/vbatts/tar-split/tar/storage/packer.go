package storage

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"unicode/utf8"
)

// ErrDuplicatePath occurs when a tar archive has more than one entry for the
// same file path
var ErrDuplicatePath = errors.New("duplicates of file paths not supported")

// Packer describes the methods to pack Entries to a storage destination
type Packer interface {
	// AddEntry packs the Entry and returns its position
	AddEntry(e Entry) (int, error)
}

// Unpacker describes the methods to read Entries from a source
type Unpacker interface {
	// Next returns the next Entry being unpacked, or error, until io.EOF
	Next() (*Entry, error)
}

type jsonUnpacker struct {
	seen seenNames
	dec  *json.Decoder
}

func (jup *jsonUnpacker) Next() (*Entry, error) {
	var e Entry
	err := jup.dec.Decode(&e)
	if err != nil {
		return nil, err
	}

	// check for dup name
	if e.Type == FileType {
		cName := filepath.Clean(e.GetName())
		if _, ok := jup.seen[cName]; ok {
			return nil, ErrDuplicatePath
		}
		jup.seen[cName] = struct{}{}
	}

	return &e, err
}

// NewJSONUnpacker provides an Unpacker that reads Entries (SegmentType and
// FileType) as a json document.
//
// Each Entry read are expected to be delimited by new line.
func NewJSONUnpacker(r io.Reader) Unpacker {
	return &jsonUnpacker{
		dec:  json.NewDecoder(r),
		seen: seenNames{},
	}
}

type jsonPacker struct {
	w    io.Writer
	e    *json.Encoder
	pos  int
	seen seenNames
}

type seenNames map[string]struct{}

func (jp *jsonPacker) AddEntry(e Entry) (int, error) {
	// if Name is not valid utf8, switch it to raw first.
	if e.Name != "" {
		if !utf8.ValidString(e.Name) {
			e.NameRaw = []byte(e.Name)
			e.Name = ""
		}
	}

	// check early for dup name
	if e.Type == FileType {
		cName := filepath.Clean(e.GetName())
		if _, ok := jp.seen[cName]; ok {
			return -1, ErrDuplicatePath
		}
		jp.seen[cName] = struct{}{}
	}

	e.Position = jp.pos
	err := jp.e.Encode(e)
	if err != nil {
		return -1, err
	}

	// made it this far, increment now
	jp.pos++
	return e.Position, nil
}

// NewJSONPacker provides a Packer that writes each Entry (SegmentType and
// FileType) as a json document.
//
// The Entries are delimited by new line.
func NewJSONPacker(w io.Writer) Packer {
	return &jsonPacker{
		w:    w,
		e:    json.NewEncoder(w),
		seen: seenNames{},
	}
}
