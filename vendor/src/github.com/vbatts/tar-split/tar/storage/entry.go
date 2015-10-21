package storage

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
	Size     int64  `json:"size,omitempty"`
	Payload  []byte `json:"payload"` // SegmentType stores payload here; FileType stores crc64 checksum here;
	Position int    `json:"position"`
}
