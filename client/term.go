package client

import (
	"syscall"
	"unsafe"
)

type Termios struct {
	Iflag  uintptr
	Oflag  uintptr
	Cflag  uintptr
	Lflag  uintptr
	Cc     [20]byte
	Ispeed uintptr
	Ospeed uintptr
}

const (
	// Input flags
	inpck  = 0x010
	istrip = 0x020
	icrnl  = 0x100
	ixon   = 0x200

	// Output flags
	opost = 0x1

	// Control flags
	cs8 = 0x300

	// Local flags
	icanon = 0x100
	iexten = 0x400
)

const (
	HUPCL   = 0x4000
	ICANON  = 0x100
	ICRNL   = 0x100
	IEXTEN  = 0x400
	BRKINT  = 0x2
	CFLUSH  = 0xf
	CLOCAL  = 0x8000
	CREAD   = 0x800
	CS5     = 0x0
	CS6     = 0x100
	CS7     = 0x200
	CS8     = 0x300
	CSIZE   = 0x300
	CSTART  = 0x11
	CSTATUS = 0x14
	CSTOP   = 0x13
	CSTOPB  = 0x400
	CSUSP   = 0x1a
	IGNBRK  = 0x1
	IGNCR   = 0x80
	IGNPAR  = 0x4
	IMAXBEL = 0x2000
	INLCR   = 0x40
	INPCK   = 0x10
	ISIG    = 0x80
	ISTRIP  = 0x20
	IUTF8   = 0x4000
	IXANY   = 0x800
	IXOFF   = 0x400
	IXON    = 0x200
	NOFLSH  = 0x80000000
	OCRNL   = 0x10
	OFDEL   = 0x20000
	OFILL   = 0x80
	ONLCR   = 0x2
	ONLRET  = 0x40
	ONOCR   = 0x20
	ONOEOT  = 0x8
	OPOST   = 0x1
	RENB    = 0x1000
	PARMRK  = 0x8
	PARODD  = 0x2000

	TOSTOP   = 0x400000
	VDISCARD = 0xf
	VDSUSP   = 0xb
	VEOF     = 0x0
	VEOL     = 0x1
	VEOL2    = 0x2
	VERASE   = 0x3
	VINTR    = 0x8
	VKILL    = 0x5
	VLNEXT   = 0xe
	VMIN     = 0x10
	VQUIT    = 0x9
	VREPRINT = 0x6
	VSTART   = 0xc
	VSTATUS  = 0x12
	VSTOP    = 0xd
	VSUSP    = 0xa
	VT0      = 0x0
	VT1      = 0x10000
	VTDLY    = 0x10000
	VTIME    = 0x11
	ECHO     = 0x00000008

	PENDIN = 0x20000000
)

type State struct {
	termios Termios
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	var termios Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd int) (*State, error) {
	var oldState State
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios)), 0, 0, 0); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= ISTRIP | INLCR | IGNCR | IXON | IXOFF
	newState.Iflag |= ICRNL
	newState.Oflag |= ONLCR
	newState.Lflag &^= ECHO | ICANON | ISIG
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func Restore(fd int, state *State) error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&state.termios)), 0, 0, 0)
	return err
}
