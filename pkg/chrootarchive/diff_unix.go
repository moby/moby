//go:build !windows

package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"io"
	"path/filepath"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/archive"
)

// applyLayerHandler parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`. Returns the size in bytes of the
// contents of the layer.
func applyLayerHandler(dest string, layer io.Reader, options *archive.TarOptions, decompress bool) (size int64, err error) {
	dest = filepath.Clean(dest)
	if decompress {
		decompressed, err := archive.DecompressStream(layer)
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
	return doUnpackLayer(dest, layer, options)
}
