package safepath

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/unix_noeintr"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Join makes sure that the concatenation of path and subpath doesn't
// resolve to a path outside of path and returns a path to a temporary file that is
// a bind mount to the exact same file/directory that was validated.
//
// After use, it is the caller's responsibility to call Close on the returned
// SafePath object, which will unmount the temporary file/directory
// and remove it.
func Join(_ context.Context, path, subpath string) (*SafePath, error) {
	base, subpart, err := evaluatePath(path, subpath)
	if err != nil {
		return nil, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	fd, err := safeOpenFd(base, subpart)
	if err != nil {
		return nil, err
	}

	defer unix_noeintr.Close(fd)

	tmpMount, err := tempMountPoint(fd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary file for safe mount")
	}

	pid := strconv.Itoa(unix.Gettid())
	// Using explicit pid path, because /proc/self/fd/<fd> fails with EACCES
	// when running under "Enhanced Container Isolation" in Docker Desktop
	// which uses sysbox runtime under the hood.
	// TODO(vvoland): Investigate.
	mountSource := "/proc/" + pid + "/fd/" + strconv.Itoa(fd)

	if err := unix_noeintr.Mount(mountSource, tmpMount, "none", unix.MS_BIND, ""); err != nil {
		os.Remove(tmpMount)
		return nil, errors.Wrap(err, "failed to mount resolved path")
	}

	return &SafePath{
		path:          tmpMount,
		sourceBase:    base,
		sourceSubpath: subpart,
		cleanup:       cleanupSafePath(tmpMount),
	}, nil
}

// safeOpenFd opens the file at filepath.Join(path, subpath) in O_PATH
// mode and returns the file descriptor if subpath is within the subtree
// rooted at path. It is an error if any of components of path or subpath
// are symbolic links.
//
// It is a caller's responsibility to close the returned file descriptor, if no
// error was returned.
func safeOpenFd(path, subpath string) (int, error) {
	// Open base volume path (_data directory).
	prevFd, err := unix_noeintr.Open(path, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, &ErrNotAccessible{Path: path, Cause: err}
	}
	defer unix_noeintr.Close(prevFd)

	// Try to use the Openat2 syscall first (available on Linux 5.6+).
	fd, err := unix_noeintr.Openat2(prevFd, subpath, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Mode:    0,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})

	switch {
	case errors.Is(err, unix.ENOSYS):
		// Openat2 is not available, fallback to Openat loop.
		return kubernetesSafeOpen(path, subpath)
	case errors.Is(err, unix.EXDEV):
		return -1, &ErrEscapesBase{Base: path, Subpath: subpath}
	case errors.Is(err, unix.ENOENT), errors.Is(err, unix.ELOOP):
		return -1, &ErrNotAccessible{Path: filepath.Join(path, subpath), Cause: err}
	case err != nil:
		return -1, &os.PathError{Op: "openat2", Path: subpath, Err: err}
	}

	// Openat2 is available and succeeded.
	return fd, nil
}

// tempMountPoint creates a temporary file/directory to act as mount
// point for the file descriptor.
func tempMountPoint(sourceFd int) (string, error) {
	var stat unix.Stat_t
	err := unix_noeintr.Fstat(sourceFd, &stat)
	if err != nil {
		return "", errors.Wrap(err, "failed to Fstat mount source fd")
	}

	isDir := (stat.Mode & unix.S_IFMT) == unix.S_IFDIR
	if isDir {
		return os.MkdirTemp("", "safe-mount")
	}

	f, err := os.CreateTemp("", "safe-mount")
	if err != nil {
		return "", err
	}

	p := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return p, nil
}

// cleanupSafePaths returns a function that unmounts the path and removes the
// mountpoint.
func cleanupSafePath(path string) func(context.Context) error {
	return func(ctx context.Context) error {
		log.G(ctx).WithField("path", path).Debug("removing safe temp mount")

		if err := unix_noeintr.Unmount(path, unix.MNT_DETACH); err != nil {
			if errors.Is(err, unix.EINVAL) {
				log.G(ctx).WithField("path", path).Warn("safe temp mount no longer exists?")
				return nil
			}
			return errors.Wrapf(err, "error unmounting safe mount %s", path)
		}
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.G(ctx).WithField("path", path).Warn("safe temp mount no longer exists?")
				return nil
			}
			return errors.Wrapf(err, "failed to delete temporary safe mount")
		}

		return nil
	}
}
