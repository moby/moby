package archive

import (
	"io"

	"github.com/moby/go-archive"
)

// Generate generates a new archive from the content provided as input.
//
// Deprecated: use [archive.Generate] instead.
func Generate(input ...string) (io.Reader, error) {
	return archive.Generate(input...)
}
