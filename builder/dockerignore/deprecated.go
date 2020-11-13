// Package dockerignore is deprecated. Use github.com/moby/buildkit/frontend/dockerfile/dockerignore instead.
package dockerignore

import (
	"io"

	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
)

// ReadAll reads a .dockerignore file and returns the list of file patterns
// to ignore. Note this will trim whitespace from each line as well
// as use GO's "clean" func to get the shortest/cleanest path for each.
//
// Deprecated: use github.com/moby/buildkit/frontend/dockerfile/dockerignore.ReadAll instead.
func ReadAll(reader io.Reader) ([]string, error) {
	return dockerignore.ReadAll(reader)
}
