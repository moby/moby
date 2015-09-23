// +build !linux,!windows

package graph

import (
	"io"

	derr "github.com/docker/docker/errors"
)

// Load method is implemented here for non-linux and non-windows platforms and
// may return an error indicating that image load is not supported on other platforms.
func (s *TagStore) Load(inTar io.ReadCloser, outStream io.Writer) error {
	return derr.ErrorCodeLoadNotSupported
}
