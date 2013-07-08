package term

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

type State struct {
	termios Termios
}

type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16
	y      uint16
}

func GetWinsize(fd uintptr) (*Winsize, error) {
	ws := &Winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	return ws, err
}

func SetWinsize(fd uintptr, ws *Winsize) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
	return err
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	var termios Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(getTermios), uintptr(unsafe.Pointer(&termios)))
	return err == 0
}

// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func RestoreTerminal(fd uintptr, state *State) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(setTermios), uintptr(unsafe.Pointer(&state.termios)))
	return err
}

func SetRawTerminal(fd uintptr) (*State, error) {
	oldState, err := MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		_ = <-c
		RestoreTerminal(fd, oldState)
		os.Exit(0)
	}()
	return oldState, err
}
