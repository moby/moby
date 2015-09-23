package storage

import (
	"bufio"
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

/* TODO(vbatts) figure out a good model for this
type PackUnpacker interface {
	Packer
	Unpacker
}
*/

type jsonUnpacker struct {
	r     io.Reader
	b     *bufio.Reader
	isEOF bool
	seen  seenNames
}

func (jup *jsonUnpacker) Next() (*Entry, error) {
	var e Entry
	if jup.isEOF {
		// since ReadBytes() will return read bytes AND an EOF, we handle it this
		// round-a-bout way so we can Unmarshal the tail with relevant errors, but
		// still get an io.EOF when the stream is ended.
		return nil, io.EOF
	}
	line, err := jup.b.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, err
	} else if err == io.EOF {
		jup.isEOF = true
	}

	err = json.Unmarshal(line, &e)
	if err != nil && jup.isEOF {
		// if the remainder actually _wasn't_ a remaining json structure, then just EOF
		return nil, io.EOF
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
		r:    r,
		b:    bufio.NewReader(r),
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

/*
TODO(vbatts) perhaps have a more compact packer/unpacker, maybe using msgapck
(https://github.com/ugorji/go)


Even though, since our jsonUnpacker and jsonPacker just take
io.Reader/io.Writer, then we can get away with passing them a
gzip.Reader/gzip.Writer
*/
