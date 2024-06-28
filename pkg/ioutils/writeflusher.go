package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"io"

	"github.com/docker/docker/internal/writeflusher"
)

// NewWriteFlusher returns a new WriteFlusher.
//
// Deprecated: use the internal/writeflusher WriteFlusher instead.
func NewWriteFlusher(w io.Writer) *writeflusher.LegacyWriteFlusher {
	return writeflusher.NewLegacyWriteFlusher(w)
}
