package console

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	cmdTcGet = unix.TCGETS
	cmdTcSet = unix.TCSETS
)

// unlockpt unlocks the slave pseudoterminal device corresponding to the master pseudoterminal referred to by f.
// unlockpt should be called before opening the slave side of a pty.
func unlockpt(f *os.File) error {
	var u int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.TIOCSPTLCK, uintptr(unsafe.Pointer(&u))); err != 0 {
		return err
	}
	return nil
}

// ptsname retrieves the name of the first available pts for the given master.
func ptsname(f *os.File) (string, error) {
	var u uint32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.TIOCGPTN, uintptr(unsafe.Pointer(&u))); err != 0 {
		return "", err
	}
	return fmt.Sprintf("/dev/pts/%d", u), nil
}
