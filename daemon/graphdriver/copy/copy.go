//go:build linux
// +build linux

package copy // import "github.com/docker/docker/daemon/graphdriver/copy"

import (
	"container/list"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/system"
	"golang.org/x/sys/unix"
)

// Mode indicates whether to use hardlink or copy content
type Mode int

const (
	// Content creates a new file, and copies the content of the file
	Content Mode = iota
	// Hardlink creates a new hardlink to the existing file
	Hardlink
)

func copyRegular(srcPath, dstPath string, fileinfo os.FileInfo, copyWithFileRange, copyWithFileClone *bool) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// If the destination file already exists, we shouldn't blow it away
	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileinfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if *copyWithFileClone {
		err = unix.IoctlFileClone(int(dstFile.Fd()), int(srcFile.Fd()))
		if err == nil {
			return nil
		}

		*copyWithFileClone = false
		if err == unix.EXDEV {
			*copyWithFileRange = false
		}
	}
	if *copyWithFileRange {
		err = doCopyWithFileRange(srcFile, dstFile, fileinfo)
		// Trying the file_clone may not have caught the exdev case
		// as the ioctl may not have been available (therefore EINVAL)
		if err == unix.EXDEV || err == unix.ENOSYS {
			*copyWithFileRange = false
		} else {
			return err
		}
	}
	return legacyCopy(srcFile, dstFile)
}

func doCopyWithFileRange(srcFile, dstFile *os.File, fileinfo os.FileInfo) error {
	amountLeftToCopy := fileinfo.Size()

	for amountLeftToCopy > 0 {
		n, err := unix.CopyFileRange(int(srcFile.Fd()), nil, int(dstFile.Fd()), nil, int(amountLeftToCopy), 0)
		if err != nil {
			return err
		}

		amountLeftToCopy = amountLeftToCopy - int64(n)
	}

	return nil
}

func legacyCopy(srcFile io.Reader, dstFile io.Writer) error {
	_, err := pools.Copy(dstFile, srcFile)

	return err
}

func copyXattr(srcPath, dstPath, attr string) error {
	data, err := system.Lgetxattr(srcPath, attr)
	if err != nil {
		return err
	}
	if data != nil {
		if err := system.Lsetxattr(dstPath, attr, data, 0); err != nil {
			return err
		}
	}
	return nil
}

type fileID struct {
	dev uint64
	ino uint64
}

type dirMtimeInfo struct {
	dstPath *string
	stat    *syscall.Stat_t
}

// DirCopy copies or hardlinks the contents of one directory to another, properly
// handling soft links, "security.capability" and (optionally) "trusted.overlay.opaque"
// xattrs.
//
// The copyOpaqueXattrs controls if "trusted.overlay.opaque" xattrs are copied.
// Passing false disables copying "trusted.overlay.opaque" xattrs.
func DirCopy(srcDir, dstDir string, copyMode Mode, copyOpaqueXattrs bool, allowXattrFailure bool) error {
	copyWithFileRange := true
	copyWithFileClone := true

	// This is a map of source file inodes to dst file paths
	copiedFiles := make(map[fileID]string)

	dirsToSetMtimes := list.New()
	err := filepath.Walk(srcDir, func(srcPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Rebase path
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		stat, ok := f.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("Unable to get raw syscall.Stat_t data for %s", srcPath)
		}

		isHardlink := false

		switch mode := f.Mode(); {
		case mode.IsRegular():
			// the type is 32bit on mips
			id := fileID{dev: uint64(stat.Dev), ino: stat.Ino} //nolint: unconvert
			if copyMode == Hardlink {
				isHardlink = true
				if err2 := os.Link(srcPath, dstPath); err2 != nil {
					return err2
				}
			} else if hardLinkDstPath, ok := copiedFiles[id]; ok {
				if err2 := os.Link(hardLinkDstPath, dstPath); err2 != nil {
					return err2
				}
			} else {
				if err2 := copyRegular(srcPath, dstPath, f, &copyWithFileRange, &copyWithFileClone); err2 != nil {
					return err2
				}
				copiedFiles[id] = dstPath
			}

		case mode.IsDir():
			if err := os.Mkdir(dstPath, f.Mode()); err != nil && !os.IsExist(err) {
				return err
			}

		case mode&os.ModeSymlink != 0:
			link, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}

			if err := os.Symlink(link, dstPath); err != nil {
				return err
			}

		case mode&os.ModeNamedPipe != 0:
			fallthrough
		case mode&os.ModeSocket != 0:
			if err := unix.Mkfifo(dstPath, stat.Mode); err != nil {
				return err
			}

		case mode&os.ModeDevice != 0:
			if userns.RunningInUserNS() {
				// cannot create a device if running in user namespace
				return nil
			}
			if err := unix.Mknod(dstPath, stat.Mode, int(stat.Rdev)); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown file type (%d / %s) for %s", f.Mode(), f.Mode().String(), srcPath)
		}

		// Everything below is copying metadata from src to dst. All this metadata
		// already shares an inode for hardlinks.
		if isHardlink {
			return nil
		}

		if err := os.Lchown(dstPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}

		if err := copyXattr(srcPath, dstPath, "security.capability"); err != nil {
			if !allowXattrFailure {
				return err
			}
		}

		if copyOpaqueXattrs {
			if err := doCopyXattrs(srcPath, dstPath); err != nil {
				return err
			}
		}

		isSymlink := f.Mode()&os.ModeSymlink != 0

		// There is no LChmod, so ignore mode for symlink. Also, this
		// must happen after chown, as that can modify the file mode
		if !isSymlink {
			if err := os.Chmod(dstPath, f.Mode()); err != nil {
				return err
			}
		}

		// system.Chtimes doesn't support a NOFOLLOW flag atm
		//nolint: unconvert
		if f.IsDir() {
			dirsToSetMtimes.PushFront(&dirMtimeInfo{dstPath: &dstPath, stat: stat})
		} else if !isSymlink {
			aTime := time.Unix(stat.Atim.Unix())
			mTime := time.Unix(stat.Mtim.Unix())
			if err := system.Chtimes(dstPath, aTime, mTime); err != nil {
				return err
			}
		} else {
			ts := []syscall.Timespec{stat.Atim, stat.Mtim}
			if err := system.LUtimesNano(dstPath, ts); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for e := dirsToSetMtimes.Front(); e != nil; e = e.Next() {
		mtimeInfo := e.Value.(*dirMtimeInfo)
		ts := []syscall.Timespec{mtimeInfo.stat.Atim, mtimeInfo.stat.Mtim}
		if err := system.LUtimesNano(*mtimeInfo.dstPath, ts); err != nil {
			return err
		}
	}

	return nil
}

func doCopyXattrs(srcPath, dstPath string) error {
	// We need to copy this attribute if it appears in an overlay upper layer, as
	// this function is used to copy those. It is set by overlay if a directory
	// is removed and then re-created and should not inherit anything from the
	// same dir in the lower dir.
	return copyXattr(srcPath, dstPath, "trusted.overlay.opaque")
}
