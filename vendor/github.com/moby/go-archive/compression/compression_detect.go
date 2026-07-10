package compression

import (
	"bytes"
	"encoding/binary"
)

const (
	zstdMagicSkippableStart = 0x184D2A50
	zstdMagicSkippableMask  = 0xFFFFFFF0
)

var (
	bzip2Magic = []byte{0x42, 0x5A, 0x68}
	gzipMagic  = []byte{0x1F, 0x8B, 0x08}
	xzMagic    = []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}
	zstdMagic  = []byte{0x28, 0xb5, 0x2f, 0xfd}
)

type matcher = func([]byte) bool

// Detect detects the compression algorithm of the source.
func Detect(source []byte) Compression {
	compressionMap := map[Compression]matcher{
		Bzip2: magicNumberMatcher(bzip2Magic),
		Gzip:  magicNumberMatcher(gzipMagic),
		Xz:    magicNumberMatcher(xzMagic),
		Zstd:  zstdMatcher(),
	}
	for _, compression := range []Compression{Bzip2, Gzip, Xz, Zstd} {
		fn := compressionMap[compression]
		if fn(source) {
			return compression
		}
	}
	return None
}

func magicNumberMatcher(m []byte) matcher {
	return func(source []byte) bool {
		return bytes.HasPrefix(source, m)
	}
}

// zstdMatcher detects zstd compression algorithm.
// Zstandard compressed data is made of one or more frames.
// There are two frame formats defined by Zstandard: Zstandard frames and Skippable frames.
// See https://datatracker.ietf.org/doc/html/rfc8878#section-3 for more details.
func zstdMatcher() matcher {
	return func(source []byte) bool {
		if bytes.HasPrefix(source, zstdMagic) {
			// Zstandard frame
			return true
		}
		// skippable frame
		if len(source) < 8 {
			return false
		}
		// magic number from 0x184D2A50 to 0x184D2A5F.
		if binary.LittleEndian.Uint32(source[:4])&zstdMagicSkippableMask == zstdMagicSkippableStart {
			return true
		}
		return false
	}
}
