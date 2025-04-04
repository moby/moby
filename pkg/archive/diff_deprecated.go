package archive

import (
	"io"

	"github.com/moby/go-archive"
)

// UnpackLayer unpack `layer` to a `dest`.
//
// Deprecated: use [archive.UnpackLayer] instead.
func UnpackLayer(dest string, layer io.Reader, options *TarOptions) (size int64, err error) {
	return archive.UnpackLayer(dest, layer, toArchiveOpt(options))
}

// ApplyLayer parses a diff in the standard layer format from `layer`,
// and applies it to the directory `dest`.
//
// Deprecated: use [archive.ApplyLayer] instead.
func ApplyLayer(dest string, layer io.Reader) (int64, error) {
	return archive.ApplyLayer(dest, layer)
}

// ApplyUncompressedLayer parses a diff in the standard layer format from
// `layer`, and applies it to the directory `dest`.
//
// Deprecated: use [archive.ApplyUncompressedLayer] instead.
func ApplyUncompressedLayer(dest string, layer io.Reader, options *TarOptions) (int64, error) {
	return archive.ApplyUncompressedLayer(dest, layer, toArchiveOpt(options))
}

// IsEmpty checks if the tar archive is empty (doesn't contain any entries).
//
// Deprecated: use [archive.IsEmpty] instead.
func IsEmpty(rd io.Reader) (bool, error) {
	return archive.IsEmpty(rd)
}
