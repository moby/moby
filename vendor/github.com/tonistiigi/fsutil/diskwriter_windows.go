//go:build windows
// +build windows

package fsutil

import (
	"fmt"
	iofs "io/fs"
	"os"
	"syscall"

	"github.com/Microsoft/go-winio"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

func rewriteMetadata(p string, stat *types.Stat) error {
	return chtimes(p, stat.ModTime)
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(path string, stat *types.Stat) error {
	return errors.New("Not implemented on windows")
}

func getFileHandle(path string, info iofs.FileInfo) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, errors.Wrap(err, "converting string to UTF-16")
	}
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	if info.Mode()&os.ModeSymlink != 0 {
		// Use FILE_FLAG_OPEN_REPARSE_POINT, otherwise CreateFile will follow symlink.
		// See https://docs.microsoft.com/en-us/windows/desktop/FileIO/symbolic-link-effects-on-file-systems-functions#createfile-and-createfiletransacted
		attrs |= syscall.FILE_FLAG_OPEN_REPARSE_POINT
	}
	h, err := syscall.CreateFile(p, 0, 0, nil, syscall.OPEN_EXISTING, attrs, 0)
	if err != nil {
		return 0, errors.Wrap(err, "getting file handle")
	}
	return h, nil
}

func readlink(path string, info iofs.FileInfo) ([]byte, error) {
	h, err := getFileHandle(path, info)
	if err != nil {
		return nil, errors.Wrap(err, "getting file handle")
	}
	defer syscall.CloseHandle(h)

	rdbbuf := make([]byte, syscall.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var bytesReturned uint32
	err = syscall.DeviceIoControl(h, syscall.FSCTL_GET_REPARSE_POINT, nil, 0, &rdbbuf[0], uint32(len(rdbbuf)), &bytesReturned, nil)
	if err != nil {
		return nil, errors.Wrap(err, "sending I/O control command")
	}
	return rdbbuf[:bytesReturned], nil
}

func getReparsePoint(path string, info iofs.FileInfo) (*winio.ReparsePoint, error) {
	target, err := readlink(path, info)
	if err != nil {
		return nil, errors.Wrap(err, "fetching link")
	}
	rp, err := winio.DecodeReparsePoint(target)
	if err != nil {
		return nil, errors.Wrap(err, "decoding reparse point")
	}
	return rp, nil
}

func renameFile(src, dst string) error {
	info, err := os.Lstat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "getting file info")
		}
	}

	if info != nil && info.Mode()&os.ModeSymlink != 0 {
		dstInfoRp, err := getReparsePoint(dst, info)
		if err != nil {
			return errors.Wrap(err, "getting reparse point")
		}
		if dstInfoRp.IsMountPoint {
			return fmt.Errorf("%s is a mount point", dst)
		}
		if err := os.Remove(dst); err != nil {
			return errors.Wrapf(err, "removing %s", dst)
		}
	}
	if err := os.Rename(src, dst); err != nil {
		return errors.Wrapf(err, "failed to rename %s to %s", src, dst)
	}
	return nil
}
