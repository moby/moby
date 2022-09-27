package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/longpath"
)

// applyLayerHandler parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`. Returns the size in bytes of the
// contents of the layer.
func applyLayerHandler(dest string, layer io.Reader, options *archive.TarOptions, decompress bool) (size int64, err error) {
	dest = filepath.Clean(dest)

	// Ensure it is a Windows-style volume path
	dest = longpath.AddPrefix(dest)

	if decompress {
		decompressed, err := archive.DecompressStream(layer)
		if err != nil {
			return 0, err
		}
		defer decompressed.Close()

		layer = decompressed
	}

	s, err := archive.UnpackLayer(dest, layer, nil)
	if err != nil {
		return 0, fmt.Errorf("ApplyLayer %s failed UnpackLayer to %s: %s", layer, dest, err)
	}

	return s, nil
}
