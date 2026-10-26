package archiveutil

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
)

const (
	zstdSkippableMagicStart = 0x184D2A50
	zstdSkippableMagicMask  = 0xFFFFFFF0
)

var (
	// bzip2 streams start with ASCII "BZh".
	// See https://www.loc.gov/preservation/digital/formats/fdd/fdd000600.shtml.
	bzip2Magic = []byte{0x42, 0x5a, 0x68}

	// gzip streams start with ID1, ID2, and compression method bytes.
	// See https://datatracker.ietf.org/doc/html/rfc1952#section-2.3.1.
	gzipMagic = []byte{0x1f, 0x8b, 0x08}

	// XZ streams start with these six header magic bytes.
	// See https://tukaani.org/xz/xz-file-format.txt.
	xzMagic = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}

	// Zstandard frames start with magic 0xFD2FB528, encoded little-endian.
	// See https://datatracker.ietf.org/doc/html/rfc8878#section-3.1.1.
	zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}
)

// IsArchive reports whether header looks like a gzip, bzip2, xz, zstd, or
// uncompressed tar archive. It does not validate the compressed contents.
func IsArchive(header []byte) bool {
	if hasBzip2Prefix(header) || HasGzipPrefix(header) || hasXZPrefix(header) || HasZstdPrefix(header) {
		return true
	}
	r := tar.NewReader(bytes.NewReader(header))
	_, err := r.Next()
	return err == nil
}

func hasBzip2Prefix(header []byte) bool {
	return bytes.HasPrefix(header, bzip2Magic)
}

// HasGzipPrefix reports whether header starts with the gzip magic bytes.
func HasGzipPrefix(header []byte) bool {
	return bytes.HasPrefix(header, gzipMagic)
}

func hasXZPrefix(header []byte) bool {
	return bytes.HasPrefix(header, xzMagic)
}

// HasZstdPrefix reports whether header starts with a zstd frame magic or a
// complete skippable frame header. It does not validate the frame contents.
func HasZstdPrefix(header []byte) bool {
	if bytes.HasPrefix(header, zstdMagic) {
		return true
	}
	// RFC 8878 section 3.1.2 defines skippable frame magic as 0x184D2A50 through 0x184D2A5F.
	// See https://datatracker.ietf.org/doc/html/rfc8878#section-3.1.2.
	return len(header) >= 8 && binary.LittleEndian.Uint32(header[:4])&zstdSkippableMagicMask == zstdSkippableMagicStart
}
