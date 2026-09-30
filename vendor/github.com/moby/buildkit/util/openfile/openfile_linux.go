package openfile

import (
	"os"
	"strconv"
	"syscall"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// openRegular pins the inode with O_PATH before deciding what it is. O_PATH
// does not call the driver's open method, so a device node is refused without
// the host device ever being opened. The pinned descriptor is then reopened
// through /proc/self/fd, so the file that is read is the inode that was
// checked, leaving no window for the path to be swapped. O_NOFOLLOW is safe
// because callers pass a path that is already fully resolved.
func openRegular(p string) (*os.File, error) {
	pinned, err := os.OpenFile(p, unix.O_PATH|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer pinned.Close()

	if err := checkRegular(pinned, p); err != nil {
		return nil, err
	}

	f, err := os.Open("/proc/self/fd/" + strconv.Itoa(int(pinned.Fd())))
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			pathErr.Path = p
		}
		return nil, errors.WithStack(err)
	}
	return f, nil
}

// openRegularInRoot resolves name within root and pins the result in one
// operation. pathrs hands back an O_PATH descriptor, so the resolved inode is
// never opened for real until it has been shown to be a regular file, and
// Reopen upgrades that exact inode rather than resolving the path again.
func openRegularInRoot(root, name string) (*os.File, error) {
	pinned, err := pathrs.OpenInRoot(root, name)
	if err != nil {
		return nil, errors.WithStack(cleanPathError(err, name))
	}
	defer pinned.Close()

	if err := checkRegular(pinned, name); err != nil {
		return nil, err
	}

	f, err := pathrs.Reopen(pinned, os.O_RDONLY)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return f, nil
}

// cleanPathError restates err as an *os.PathError naming name. Resolution
// errors from pathrs quote the daemon-side path they failed on, which must not
// reach the client; the errno is preserved so errors.Is still identifies it.
func cleanPathError(err error, name string) error {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return &os.PathError{Op: "open", Path: name, Err: errno}
	}
	// pathrs' pre-openat2 fallback reports breakout detections that are not
	// errnos, and whose diagnostics can quote the path they resolved. Keep the
	// EXDEV signal those carry and drop everything else.
	if errors.Is(err, unix.EXDEV) {
		return &os.PathError{Op: "open", Path: name, Err: unix.EXDEV}
	}
	return &os.PathError{Op: "open", Path: name, Err: os.ErrInvalid}
}
