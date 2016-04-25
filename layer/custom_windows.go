package layer

import (
	"io"

	"github.com/docker/docker/pkg/wimutils"
)

// TarFromNonTar parses a file to see if it a non-tar layer. If so, it returns
// a tar stream with the layer's contents. If not, it returns nil.
func TarFromNonTar(f io.ReaderAt, size int64) (io.ReadCloser, int64, error) {
	isWIM, err := wimutils.IsWIM(f)
	if err != nil {
		return nil, 0, err
	}
	if !isWIM {
		return nil, size, nil
	}

	t, err := wimutils.TarFromWIM(f)
	if err != nil {
		return nil, 0, err
	}

	return t, 0, err // The size cannot be computed for this tar without reading the whole thing.
}
