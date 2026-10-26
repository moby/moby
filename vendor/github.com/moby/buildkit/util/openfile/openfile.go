// Package openfile opens files that a build may have placed in a snapshot.
//
// A snapshot is attacker-controlled: an ExecOp can mknod a device node or
// mkfifo with the default capability set. Opening such an inode from the daemon
// resolves it against the host, so the daemon would act on a device the sandbox
// itself is denied, or block indefinitely in open(2) on a fifo with no writer.
// It refuses anything that is not a regular file.
package openfile

import (
	"os"

	"github.com/pkg/errors"
)

// ErrNotRegular is reported when the path exists but is not a regular file.
var ErrNotRegular = errors.New("not a regular file")

// Regular opens p for reading and fails unless p is a regular file. Callers are
// expected to have resolved p within the snapshot already, for example with
// containerd/continuity fs.RootPath.
func Regular(p string) (*os.File, error) {
	return openRegular(p)
}

// RegularInRoot opens name inside root for reading and fails unless it is a
// regular file. Unlike Regular it resolves name itself, confined to root, and
// does so as a single operation: a process running against the same mount
// cannot swap a path component between the resolution and the open. Use it
// when the mount may be mutated while it is read, as a gateway container mount
// can be.
func RegularInRoot(root, name string) (*os.File, error) {
	return openRegularInRoot(root, name)
}

func checkRegular(f *os.File, name string) error {
	fi, err := f.Stat()
	if err != nil {
		return errors.WithStack(err)
	}
	if !fi.Mode().IsRegular() {
		return errors.WithStack(&os.PathError{Op: "open", Path: name, Err: ErrNotRegular})
	}
	return nil
}
