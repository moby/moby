package term

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	getTermios = unix.TIOCGETA
	setTermios = unix.TIOCSETA
)

// Termios magic numbers, passthrough to the ones defined in unix.
const (
	IGNBRK = unix.IGNBRK
	PARMRK = unix.PARMRK
	INLCR  = unix.INLCR
	IGNCR  = unix.IGNCR
	ECHONL = unix.ECHONL
	CSIZE  = unix.CSIZE
	ICRNL  = unix.ICRNL
	ISTRIP = unix.ISTRIP
	PARENB = unix.PARENB
	ECHO   = unix.ECHO
	ICANON = unix.ICANON
	ISIG   = unix.ISIG
	IXON   = unix.IXON
	BRKINT = unix.BRKINT
	INPCK  = unix.INPCK
	OPOST  = unix.OPOST
	CS8    = unix.CS8
	IEXTEN = unix.IEXTEN
)

// Termios is the Unix API for terminal I/O.
type Termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Cc     [20]byte
	Ispeed uint32
	Ospeed uint32
}

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	var oldState State
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios))); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= (IGNBRK | BRKINT | PARMRK | ISTRIP | INLCR | IGNCR | ICRNL | IXON)
	newState.Oflag &^= OPOST
	newState.Lflag &^= (ECHO | ECHONL | ICANON | ISIG | IEXTEN)
	newState.Cflag &^= (CSIZE | PARENB)
	newState.Cflag |= CS8
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0

	if _, _, err := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(setTermios), uintptr(unsafe.Pointer(&newState))); err != 0 {
		return nil, err
	}

	return &oldState, nil
}
