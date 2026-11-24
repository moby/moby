//go:build !linux && !darwin && !freebsd && !netbsd

package archive

import (
	"os"

	"github.com/moby/go-archive/internal/archiveoptions"
)

func withXattrPath(*os.Root, string, *archiveoptions.Options, func(string) error) error {
	return nil
}

func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}

func lsetxattr(path string, attr string, data []byte, flags int) error {
	return nil
}
