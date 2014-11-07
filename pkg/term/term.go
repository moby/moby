package term

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

// #include <termios.h>
import "C"

var (
	ErrInvalidState = errors.New("Invalid terminal state")
)

type Termios syscall.Termios

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
	// Skipp errno = 0
	if err == 0 {
		return ws, nil
	}
	return ws, err
}

func SetWinsize(fd uintptr, ws *Winsize) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
	// Skipp errno = 0
	if err == 0 {
		return nil
	}
	return err
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	var termios Termios
	err := tcget(fd, &termios)
	return err == 0
}

// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func RestoreTerminal(fd uintptr, state *State) error {
	if state == nil {
		return ErrInvalidState
	}
	err := tcset(fd, &state.termios)
	if err != 0 {
		return err
	}
	return nil
}

func SaveState(fd uintptr) (*State, error) {
	var oldState State
	if err := tcget(fd, &oldState.termios); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

func DisableEcho(fd uintptr, state *State) error {
	newState := state.termios
	newState.Lflag &^= syscall.ECHO

	if err := tcset(fd, &newState); err != 0 {
		return err
	}
	handleInterrupt(fd, state)
	return nil
}

func SetRawTerminal(fd uintptr) (*State, error) {
	oldState, err := MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	handleInterrupt(fd, oldState)
	return oldState, err
}

func handleInterrupt(fd uintptr, state *State) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	go func() {
		_ = <-sigchan
		RestoreTerminal(fd, state)
		os.Exit(0)
	}()
}

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	var oldState State
	if err := tcget(fd, &oldState.termios); err != 0 {
		return nil, err
	}

	newState := oldState.termios

	C.cfmakeraw((*C.struct_termios)(unsafe.Pointer(&newState)))
	if err := tcset(fd, &newState); err != 0 {
		return nil, err
	}
	return &oldState, nil
}

func tcget(fd uintptr, p *Termios) syscall.Errno {
	ret, err := C.tcgetattr(C.int(fd), (*C.struct_termios)(unsafe.Pointer(p)))
	if ret != 0 {
		return err.(syscall.Errno)
	}
	return 0
}

func tcset(fd uintptr, p *Termios) syscall.Errno {
	ret, err := C.tcsetattr(C.int(fd), C.TCSANOW, (*C.struct_termios)(unsafe.Pointer(p)))
	if ret != 0 {
		return err.(syscall.Errno)
	}
	return 0
}
