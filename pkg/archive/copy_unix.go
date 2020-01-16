// +build !windows

package archive // import "github.com/moby/moby/pkg/archive"

import (
	"path/filepath"
)

func normalizePath(path string) string {
	return filepath.ToSlash(path)
}
