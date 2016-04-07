// +build !windows

package layer

import "io"

// TarFromNonTar parses a file to see if it a non-tar layer. If so, it returns
// a tar stream with the layer's contents. If not, it returns nil.
func TarFromNonTar(_ io.ReaderAt, size int64) (io.ReadCloser, int64, error) {
	return nil, size, nil
}
