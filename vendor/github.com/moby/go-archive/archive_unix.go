//go:build !windows

package archive

import (
	"archive/tar"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

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
func chmodTarEntry(perm os.FileMode) os.FileMode {
	return perm // noop for unix as golang APIs provide perm bits correctly
}

func getInodeFromStat(stat interface{}) (uint64, error) {
	s, ok := stat.(*syscall.Stat_t)
	if !ok {
		// FIXME(thaJeztah): this should likely return an error; see https://github.com/moby/moby/pull/49493#discussion_r1979152897
		return 0, nil
	}
	return s.Ino, nil
}

func getFileUIDGID(stat interface{}) (int, int, error) {
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
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {
	mode := uint32(hdr.Mode & 0o7777)
	switch hdr.Typeflag {
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	}

	return mknod(path, mode, unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor)))
}

// handleLChmod applies the mode from hdrInfo to path within root using dc,
// skipping symlinks (there is no lchmod). For hardlinks, the mode is applied
// only when the link target is itself not a symlink.
func handleLChmod(dc *dirCache, root *os.Root, path string, hdr *tar.Header, hdrInfo os.FileInfo) error {
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := root.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := dc.chmod(root, path, hdrInfo.Mode()); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := dc.chmod(root, path, hdrInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}
