//go:build !linux

package errdefs

import "syscall"

func syscallErrors() map[syscall.Errno]bool {
	return nil
}
