package stdcopy // Deprecated: use [github.com/moby/moby/api/stdcopy] instead.

import (
	"io"

	"github.com/moby/moby/api/stdcopy"
)

// StdType is the type of standard stream
// a writer can multiplex to.
//
// Deprecated: use [stdcopy.StdType]. This alias will be removed in the next release.
type StdType = stdcopy.StdType

const (
	Stdin     = stdcopy.Stdin     // Deprecated: use [stdcopy.Stderr]. This alias will be removed in the next release.
	Stdout    = stdcopy.Stdout    // Deprecated: use [stdcopy.Stdout]. This alias will be removed in the next release.
	Stderr    = stdcopy.Stderr    // Deprecated: use [stdcopy.Stderr]. This alias will be removed in the next release.
	Systemerr = stdcopy.Systemerr // Deprecated: use [stdcopy.Systemerr]. This alias will be removed in the next release.
)

// NewStdWriter instantiates a new Writer.
//
// Deprecated: use [stdcopy.NewStdWriter]. This alias will be removed in the next release.
func NewStdWriter(w io.Writer, t stdcopy.StdType) io.Writer {
	return stdcopy.NewStdWriter(w, t)
}

// StdCopy is a modified version of io.Copy.
//
// Deprecated: use [stdcopy.StdCopy]. This alias will be removed in the next release.
func StdCopy(dstout, dsterr io.Writer, src io.Reader) (written int64, _ error) {
	return stdcopy.StdCopy(dstout, dsterr, src)
}
