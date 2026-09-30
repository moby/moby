//go:build !linux

package openfile

import (
	"os"

	"github.com/containerd/continuity/fs"

	"github.com/pkg/errors"
)

// openRegular opens without blocking so that a fifo cannot stall the daemon,
// then refuses anything that is not a regular file. Unlike the Linux
// implementation this does invoke a device driver's open method, as O_PATH has
// no portable equivalent.
func openRegular(p string) (*os.File, error) {
	// Decide on the link itself first. Lstat invokes no device driver, so a
	// special file already sitting at p is refused without one running, and the
	// caller gets ErrNotRegular rather than whatever open would have reported.
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.WithStack(&os.PathError{Op: "open", Path: p, Err: ErrNotRegular})
	}

	f, err := os.OpenFile(p, os.O_RDONLY|nonBlock|noFollow, 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := checkRegular(f, p); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// openRegularInRoot has no race-safe equivalent off Linux: resolution and open
// are separate steps, so a mount mutated in between can still redirect the
// open. The inode type is still checked before and after opening.
func openRegularInRoot(root, name string) (*os.File, error) {
	p, err := fs.RootPath(root, name)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return openRegular(p)
}
