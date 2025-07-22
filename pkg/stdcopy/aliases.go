package stdcopy

import (
	"io"

	"github.com/moby/moby/api/pkg/stdcopy"
)

// StdType is the type of standard stream
// a writer can multiplex to.
//
// Deprecated: use [stdcopy.StdType]. This alias will be removed in the next release.
type StdType = stdcopy.StdType

const (
	Stdin     = stdcopy.Stdin
	Stdout    = stdcopy.Stdout
	Stderr    = stdcopy.Stderr
	Systemerr = stdcopy.Systemerr
)

// NewStdWriter instantiates a new Writer.
func NewStdWriter(w io.Writer, t stdcopy.StdType) io.Writer {
	return stdcopy.NewStdWriter(w, t)
}

// StdCopy is a modified version of io.Copy.
func StdCopy(dstout, dsterr io.Writer, src io.Reader) (written int64, _ error) {
	return stdcopy.StdCopy(dstout, dsterr, src)
}
