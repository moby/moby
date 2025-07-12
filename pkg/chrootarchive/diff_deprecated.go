package chrootarchive

import (
	"io"

	"github.com/moby/go-archive/chrootarchive"

	"github.com/docker/docker/pkg/archive"
)

// ApplyLayer parses a diff in the standard layer format from `layer`,
// and applies it to the directory `dest`.
//
// Deprecated: use [chrootarchive.ApplyLayer] insteead.
func ApplyLayer(dest string, layer io.Reader) (size int64, err error) {
	return chrootarchive.ApplyLayer(dest, layer)
}

// ApplyUncompressedLayer parses a diff in the standard layer format from
// `layer`, and applies it to the directory `dest`.
//
// Deprecated: use [chrootarchive.ApplyUncompressedLayer] insteead.
func ApplyUncompressedLayer(dest string, layer io.Reader, options *archive.TarOptions) (int64, error) {
	return chrootarchive.ApplyUncompressedLayer(dest, layer, archive.ToArchiveOpt(options))
}
