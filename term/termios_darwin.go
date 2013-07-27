package term

import (
	"syscall"
	"unsafe"
)

const (
	getTermios = syscall.TIOCGETA
	setTermios = syscall.TIOCSETA

	ECHO   = 0x00000008
	ONLCR  = 0x2
	ISTRIP = 0x20
	INLCR  = 0x40
	ISIG   = 0x80
	IGNCR  = 0x80
	ICANON = 0x100
	ICRNL  = 0x100
	IXOFF  = 0x400
	IXON   = 0x200
)

type Termios struct {
	Iflag  uint64
	Oflag  uint64
	Cflag  uint64
	Lflag  uint64
	Cc     [20]byte
	Ispeed uint64
	Ospeed uint64
}

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	var oldState State
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios))); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= (ISTRIP | INLCR | IGNCR | IXON | IXOFF)
	newState.Iflag |= ICRNL
	newState.Oflag |= ONLCR
	newState.Lflag &^= (ECHO | ICANON)

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(setTermios), uintptr(unsafe.Pointer(&newState))); err != 0 {
		return nil, err
	}

	return &oldState, nil
}
