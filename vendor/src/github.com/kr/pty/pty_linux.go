package pty

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

var (
	ioctl_TIOCGPTN   = _IOR('T', 0x30, unsafe.Sizeof(_C_uint(0))) /* Get Pty Number (of pty-mux device) */
	ioctl_TIOCSPTLCK = _IOW('T', 0x31, unsafe.Sizeof(_C_int(0)))  /* Lock/unlock Pty */
)

func open() (pty, tty *os.File, err error) {
	p, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	sname, err := ptsname(p)
	if err != nil {
		return nil, nil, err
	}

	err = unlockpt(p)
	if err != nil {
		return nil, nil, err
	}

	t, err := os.OpenFile(sname, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return p, t, nil
}

func ptsname(f *os.File) (string, error) {
	var n _C_uint
	err := ioctl(f.Fd(), ioctl_TIOCGPTN, uintptr(unsafe.Pointer(&n)))
	if err != nil {
		return "", err
	}
	return "/dev/pts/" + strconv.Itoa(int(n)), nil
}

func unlockpt(f *os.File) error {
	var u _C_int
	// use TIOCSPTLCK with a zero valued arg to clear the slave pty lock
	return ioctl(f.Fd(), ioctl_TIOCSPTLCK, uintptr(unsafe.Pointer(&u)))
}
