//+build solaris illumos

package term

import (
	"golang.org/x/sys/unix"
	"syscall"
)

func tcget(fd uintptr, p *Termios) syscall.Errno {

	termios, err := unix.IoctlGetTermios(int(fd), getTermios)
	if err != nil {
		return syscall.EINVAL
	}
	p = (*Termios)(termios)
	return 0
}

func tcset(fd uintptr, p *Termios) syscall.Errno {
	if err := unix.IoctlSetTermios(int(fd), setTermios, (*unix.Termios)(p)); err != nil {
		return syscall.EINVAL
	}
	return 0
}
