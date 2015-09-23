package storage

import "unicode/utf8"

// Entries is for sorting by Position
type Entries []Entry

func (e Entries) Len() int           { return len(e) }
func (e Entries) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e Entries) Less(i, j int) bool { return e[i].Position < e[j].Position }

// Type of Entry
type Type int

const (
	// FileType represents a file payload from the tar stream.
	//
	// This will be used to map to relative paths on disk. Only Size > 0 will get
	// read into a resulting output stream (due to hardlinks).
	FileType Type = 1 + iota
	// SegmentType represents a raw bytes segment from the archive stream. These raw
	// byte segments consist of the raw headers and various padding.
	//
	// Its payload is to be marshalled base64 encoded.
	SegmentType
)

// Entry is the structure for packing and unpacking the information read from
// the Tar archive.
//
// FileType Payload checksum is using `hash/crc64` for basic file integrity,
// _not_ for cryptography.
// From http://www.backplane.com/matt/crc64.html, CRC32 has almost 40,000
// collisions in a sample of 18.2 million, CRC64 had none.
type Entry struct {
	Type     Type   `json:"type"`
	Name     string `json:"name,omitempty"`
	NameRaw  []byte `json:"name_raw,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Payload  []byte `json:"payload"` // SegmentType stores payload here; FileType stores crc64 checksum here;
	Position int    `json:"position"`
}

// SetName will check name for valid UTF-8 string, and set the appropriate
// field. See https://github.com/vbatts/tar-split/issues/17
func (e *Entry) SetName(name string) {
	if utf8.ValidString(name) {
		e.Name = name
	} else {
		e.NameRaw = []byte(name)
	}
}

// SetNameBytes will check name for valid UTF-8 string, and set the appropriate
// field
func (e *Entry) SetNameBytes(name []byte) {
	if utf8.Valid(name) {
		e.Name = string(name)
	} else {
		e.NameRaw = name
	}
}

// GetName returns the string for the entry's name, regardless of the field stored in
func (e *Entry) GetName() string {
	if len(e.NameRaw) > 0 {
		return string(e.NameRaw)
	}
	return e.Name
}

// GetNameBytes returns the bytes for the entry's name, regardless of the field stored in
func (e *Entry) GetNameBytes() []byte {
	if len(e.NameRaw) > 0 {
		return e.NameRaw
	}
	return []byte(e.Name)
}
