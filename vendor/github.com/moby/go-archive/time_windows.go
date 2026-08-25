package archive

import (
	"errors"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// chtimes changes the access and modification time of a file at the given
// path relative to root.
//
// Symlink entries are handled separately through lchtimes. The final path
// component is expected not to be a reparse point; if one is encountered,
// chtimes returns an error.
//
// Callers must use boundTime to ensure timestamps are within the range
// supported by os.Chtimes.
func chtimes(root *os.Root, name string, atime, mtime time.Time) error {
	parent, err := root.OpenFile(filepath.Dir(name), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer parent.Close()

	// Symlink entries are handled by lchtimes. The destination for all
	// chtimes callers is therefore expected not to be a reparse point.
	//
	// Do not follow the final component: if it was concurrently replaced
	// with a reparse point, fail instead of updating its target.
	return chtimesAt(parent, filepath.Base(name), atime, mtime, true)
}

func lchtimes(root *os.Root, name string, atime time.Time, mtime time.Time) error {
	return nil
}

func chtimesAt(parent *os.File, name string, atime, mtime time.Time, noFollow bool) error {
	h, err := openForWriteAttributesAt(windows.Handle(parent.Fd()), name, noFollow)
	if err != nil {
		if noFollow && errors.Is(err, windows.STATUS_REPARSE_POINT_ENCOUNTERED) {
			// Encountering a reparse point when noFollow is requested is unexpected.
			// Treat it as a potential breakout to fail extraction safely.
			return breakoutError(err)
		}
		return err
	}
	defer func() { _ = windows.Close(h) }()

	var (
		creationTime     = windows.NsecToFiletime(mtime.UnixNano())
		accessTime       = windows.NsecToFiletime(atime.UnixNano())
		modificationTime = windows.NsecToFiletime(mtime.UnixNano())
	)
	return windows.SetFileTime(h, &creationTime, &accessTime, &modificationTime)
}

// openForWriteAttributesAt opens name relative to parent with permission to
// modify its file attributes. If noFollow is true, it does not follow reparse
// points.
//
// This implementation is based on Go's internal Windows Openat support:
//
//	https://github.com/golang/go/blob/go1.26.0/src/internal/syscall/windows/at_windows.go
//
// It is used by os.Root's Windows implementation for root-relative filesystem
// operations:
//
//	https://github.com/golang/go/blob/go1.26.0/src/os/root_windows.go
//
// Keep this implementation aligned with the upstream code until an equivalent
// operation is available from golang.org/x/sys/windows.
func openForWriteAttributesAt(parent windows.Handle, name string, noFollow bool) (windows.Handle, error) {
	name16, err := windows.UTF16FromString(name)
	if err != nil {
		return windows.InvalidHandle, err
	}

	attrs := uint32(windows.OBJ_CASE_INSENSITIVE)
	if noFollow {
		attrs |= windows.OBJ_DONT_REPARSE
	}

	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		windows.SYNCHRONIZE|windows.FILE_WRITE_ATTRIBUTES,
		&windows.OBJECT_ATTRIBUTES{
			Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
			RootDirectory: parent,
			ObjectName: &windows.NTUnicodeString{
				Length:        uint16((len(name16) - 1) * 2), // #nosec G115 -- Length is USHORT by definition. A Windows path component cannot exceed uint16 bytes.
				MaximumLength: uint16(len(name16) * 2),       // #nosec G115 -- MaximumLength is USHORT by definition. A Windows path component cannot exceed uint16 bytes.
				Buffer:        &name16[0],
			},
			Attributes: attrs,
		},
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0, // EA buffer
		0, // EA length
	)
	return handle, err
}
