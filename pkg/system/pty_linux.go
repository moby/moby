package system

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Unlockpt unlocks the slave pseudoterminal device corresponding to the master pseudoterminal referred to by f.
// Unlockpt should be called before opening the slave side of a pseudoterminal.
func Unlockpt(f *os.File) error {
	var u int
	return Ioctl(f.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&u)))
}

// Ptsname retrieves the name of the first available pts for the given master.
func Ptsname(f *os.File) (string, error) {
	var n int

	if err := Ioctl(f.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		return "", err
	}
	return fmt.Sprintf("/dev/pts/%d", n), nil
}

// OpenPtmx opens /dev/ptmx, i.e. the PTY master.
func OpenPtmx() (*os.File, error) {
	// O_NOCTTY and O_CLOEXEC are not present in os package so we use the syscall's one for all.
	return os.OpenFile("/dev/ptmx", syscall.O_RDONLY|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
}
