//go:build !windows

package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var errInvalidArchive = errors.New("invalid archive")

// addLongPathPrefix adds the Windows long path prefix to the path provided if
// it does not already have it. It is a no-op on platforms other than Windows.
func addLongPathPrefix(srcPath string) string {
	return srcPath
}

// getWalkRoot calculates the root path when performing a TarWithOptions.
// We use a separate function as this is platform specific. On Linux, we
// can't use filepath.Join(srcPath,include) because this will clean away
// a trailing "." or "/" which may be important.
func getWalkRoot(srcPath string, include string) string {
	return strings.TrimSuffix(srcPath, string(filepath.Separator)) + string(filepath.Separator) + include
}

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.
func chmodTarEntry(mode int64) int64 {
	return mode // noop for unix as golang APIs provide perm bits correctly
}

func getInodeFromStat(stat any) (uint64, error) {
	s, ok := stat.(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unexpected stat type %T", stat)
	}
	return s.Ino, nil
}

func getFileUIDGID(stat any) (int, int, error) {
	s, ok := stat.(*syscall.Stat_t)

	if !ok {
		return 0, 0, errors.New("cannot convert stat value to syscall.Stat_t")
	}
	return int(s.Uid), int(s.Gid), nil
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo.
//
// Creating device nodes is not supported when running in a user namespace,
// produces a [syscall.EPERM] in most cases.
func handleTarTypeBlockCharFifo(root *os.Root, hdr *tar.Header, dstPath string) error {
	mode := uint32(hdr.Mode & 0o7777)
	switch hdr.Typeflag {
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	}

	// Devmajor and Devminor come straight from the (untrusted) tar header as
	// int64, but Mkdev only takes uint32. Casting a value that does not fit
	// silently truncates it, so the node created on disk would carry a
	// different major/minor than the header declares. Reject those instead of
	// creating a mismatched device.
	if hdr.Devmajor < 0 || hdr.Devmajor > math.MaxUint32 ||
		hdr.Devminor < 0 || hdr.Devminor > math.MaxUint32 {
		return fmt.Errorf("device number %d:%d for %q out of range: %w", hdr.Devmajor, hdr.Devminor, hdr.Name, errInvalidArchive)
	}

	// Prefer mknodat; fall back to a bounded path where unavailable.
	return mknodInRoot(root, dstPath, mode, unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor)))
}

// handleLChmod applies the mode from hdrInfo to dstPath within root, skipping
// symlinks (there is no lchmod). For hardlinks, the mode is applied only when
// the link target is itself not a symlink.
func handleLChmod(root *os.Root, dstPath string, hdr *tar.Header, hdrInfo os.FileInfo) error {
	switch hdr.Typeflag {
	case tar.TypeSymlink:
		return nil

	case tar.TypeLink:
		// If the target is a symlink, there is no way to chmod the hardlink
		// without following it.
		fi, err := root.Lstat(filepath.FromSlash(path.Clean(hdr.Linkname)))
		if err != nil || fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		return chmodNoSymlink(root, dstPath, hdrInfo.Mode())

	default:
		return chmodNoSymlink(root, dstPath, hdrInfo.Mode())
	}
}

// chmodNoSymlink applies mode to a non-symlink entry.
//
// Callers must have already excluded symlink entries.
func chmodNoSymlink(root *os.Root, name string, mode os.FileMode) error {
	parent, err := root.OpenFile(filepath.Dir(name), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer parent.Close()

	base := filepath.Base(name)
	perm := fileModeToPerm(mode)
	// #nosec G115 -- ignore integer overflow conversion for parent.Fd
	if err := unix.Fchmodat(int(parent.Fd()), base, perm, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EOPNOTSUPP) {
		return &os.PathError{Op: "fchmodat2", Path: name, Err: err}
	}

	// Fallback for systems that cannot perform fchmodat with AT_SYMLINK_NOFOLLOW.
	// Open the entry without following symlinks and apply the mode through the
	// resulting file descriptor.
	// #nosec G115 -- ignore integer overflow conversion for parent.Fd
	fd, err := unix.Openat(int(parent.Fd()), base, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return &os.PathError{Op: "openat", Path: name, Err: err}
	}
	defer unix.Close(fd)

	if err := unix.Fchmod(fd, perm); err != nil {
		return &os.PathError{Op: "fchmod", Path: name, Err: err}
	}
	return nil
}

// fileModeToPerm returns the subset of an os.FileMode that can be applied
// by chmod.
func fileModeToPerm(mode os.FileMode) uint32 {
	perm := uint32(mode.Perm())

	if mode&os.ModeSetuid != 0 {
		perm |= unix.S_ISUID
	}
	if mode&os.ModeSetgid != 0 {
		perm |= unix.S_ISGID
	}
	if mode&os.ModeSticky != 0 {
		perm |= unix.S_ISVTX
	}
	return perm
}
