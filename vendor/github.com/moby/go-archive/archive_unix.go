//go:build !windows

package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"math"
	"os"
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
func handleTarTypeBlockCharFifo(hdr *tar.Header, dstPath string) error {
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

	return mknod(dstPath, mode, unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor)))
}

func handleLChmod(hdr *tar.Header, dstPath string, hdrInfo os.FileInfo) error {
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := os.Chmod(dstPath, hdrInfo.Mode()); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := os.Chmod(dstPath, hdrInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}
