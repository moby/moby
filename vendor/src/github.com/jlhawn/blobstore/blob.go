package blobstore

import (
	"io"
	"os"
)

type blob struct {
	Descriptor
	blobFilepath string
}

func newBlob(d Descriptor, blobFilepath string) Blob {
	return &blob{Descriptor: d, blobFilepath: blobFilepath}
}

// Open should open the underlying blob for reading. It is the responsibility
// of the caller to close the returned io.ReadCloser. Returns a nil error on
// success.
func (h *blob) Open() (io.ReadCloser, error) {
	return os.Open(h.blobFilepath)
}
