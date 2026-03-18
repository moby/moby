package sysfs

import (
	"os"
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func rename(from, to string) sys.Errno {
	if from == to {
		return 0
	}

	var fromIsDir, toIsDir bool
	if fromStat, errno := stat(from); errno != 0 {
		return errno // failed to stat from
	} else {
		fromIsDir = fromStat.Mode.IsDir()
	}
	if toStat, errno := stat(to); errno == sys.ENOENT {
		return syscallRename(from, to) // file or dir to not-exist is ok
	} else if errno != 0 {
		return errno // failed to stat to
	} else {
		toIsDir = toStat.Mode.IsDir()
	}

	// Now, handle known cases
	switch {
	case !fromIsDir && toIsDir: // file to dir
		return sys.EISDIR
	case !fromIsDir && !toIsDir: // file to file
		// Use os.Rename instead of syscall.Rename to overwrite a file.
		// This uses MoveFileEx instead of MoveFile (used by syscall.Rename).
		return sys.UnwrapOSError(os.Rename(from, to))
	case fromIsDir && !toIsDir: // dir to file
		return sys.ENOTDIR
	default: // dir to dir

		// We can't tell if a directory is empty or not, via stat information.
		// Reading the directory is expensive, as it can buffer large amounts
		// of data on fail. Instead, speculatively try to remove the directory.
		// This is only one syscall and won't buffer anything.
		if errno := rmdir(to); errno == 0 || errno == sys.ENOENT {
			return syscallRename(from, to)
		} else {
			return errno
		}
	}
}

func syscallRename(from string, to string) sys.Errno {
	return sys.UnwrapOSError(syscall.Rename(from, to))
}
