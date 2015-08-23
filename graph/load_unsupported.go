// +build !linux,!windows

package graph

import (
	"fmt"
	"io"
)

// Load method is implemented here for non-linux and non-windows platforms and
// may return an error indicating that image load is not supported on other platforms.
func (s *TagStore) Load(inTar io.ReadCloser, outStream io.Writer) error {
	return fmt.Errorf("Load is not supported on this platform")
}
