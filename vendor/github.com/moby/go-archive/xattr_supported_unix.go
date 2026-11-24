//go:build darwin || freebsd || netbsd

package archive

import (
	"os"
	"path/filepath"

	"github.com/moby/go-archive/internal/archiveoptions"
	"golang.org/x/sys/unix"
)

var noattr = unix.ENOATTR

func withXattrPath(root *os.Root, name string, _ *archiveoptions.Options, apply func(string) error) error {
	parent, err := fsRootPath(root.Name(), filepath.Dir(name))
	if err != nil {
		return err
	}
	return apply(filepath.Join(parent, filepath.Base(name)))
}
