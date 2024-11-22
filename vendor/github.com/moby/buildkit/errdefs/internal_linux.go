//go:build linux

package errdefs

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// syscallErrors returns a map of syscall errors that are considered internal.
// value is true if the error is of type resource exhaustion, false otherwise.
func syscallErrors() map[syscall.Errno]bool {
	return map[syscall.Errno]bool{
		unix.EIO:             false, // I/O error
		unix.ENOMEM:          true,  // Out of memory
		unix.EFAULT:          false, // Bad address
		unix.ENOSPC:          true,  // No space left on device
		unix.ENOTRECOVERABLE: false, // State not recoverable
		unix.EHWPOISON:       false, // Memory page has hardware error
	}
}
