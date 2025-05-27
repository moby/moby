//go:build !windows

package chrootarchive

import (
	"io"
	"path/filepath"

	"github.com/moby/sys/userns"

	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
)

// applyLayerHandler parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`. Returns the size in bytes of the
// contents of the layer.
func applyLayerHandler(dest string, layer io.Reader, options *archive.TarOptions, decompress bool) (size int64, _ error) {
	if decompress {
		decompressed, err := compression.DecompressStream(layer)
		if err != nil {
			return 0, err
		}
		defer decompressed.Close()

		layer = decompressed
	}
	if options == nil {
		options = &archive.TarOptions{}
	}
	if userns.RunningInUserNS() {
		options.InUserNS = true
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}
	dest = filepath.Clean(dest)
	return doUnpackLayer(dest, layer, options)
}
