package sysfs

import (
	"io"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func adjustReaddirErr(f sys.File, isClosed bool, err error) sys.Errno {
	if err == io.EOF {
		return 0 // e.g. Readdir on darwin returns io.EOF, but linux doesn't.
	} else if errno := sys.UnwrapOSError(err); errno != 0 {
		errno = dirError(f, isClosed, errno)
		// Comply with errors allowed on sys.File Readdir
		switch errno {
		case sys.EINVAL: // os.File Readdir can return this
			return sys.EBADF
		case sys.ENOTDIR: // dirError can return this
			return sys.EBADF
		}
		return errno
	}
	return 0
}
