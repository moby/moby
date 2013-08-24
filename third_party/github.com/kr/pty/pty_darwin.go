package pty

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// see ioccom.h
const sys_IOCPARM_MASK = 0x1fff

func open() (pty, tty *os.File, err error) {
	p, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	sname, err := ptsname(p)
	if err != nil {
		return nil, nil, err
	}

	err = grantpt(p)
	if err != nil {
		return nil, nil, err
	}

	err = unlockpt(p)
	if err != nil {
		return nil, nil, err
	}

	t, err := os.OpenFile(sname, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	return p, t, nil
}

func ptsname(f *os.File) (string, error) {
	var n [(syscall.TIOCPTYGNAME >> 16) & sys_IOCPARM_MASK]byte

	ioctl(f.Fd(), syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&n)))
	for i, c := range n {
		if c == 0 {
			return string(n[:i]), nil
		}
	}
	return "", errors.New("TIOCPTYGNAME string not NUL-terminated")
}

func grantpt(f *os.File) error {
	var u int
	return ioctl(f.Fd(), syscall.TIOCPTYGRANT, uintptr(unsafe.Pointer(&u)))
}

func unlockpt(f *os.File) error {
	var u int
	return ioctl(f.Fd(), syscall.TIOCPTYUNLK, uintptr(unsafe.Pointer(&u)))
}

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if e != 0 {
		return syscall.ENOTTY
	}
	return nil
}
