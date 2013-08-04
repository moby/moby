package term

import (
	"fmt"
	"io"
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

func SaveState(fd uintptr) (*State, error) {
	var oldState State
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, getTermios, uintptr(unsafe.Pointer(&oldState.termios))); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

func DisableEcho(fd uintptr, out io.Writer, state *State) error {
	newState := state.termios
	newState.Lflag &^= syscall.ECHO

	HandleInterrupt(fd, out, state)
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, setTermios, uintptr(unsafe.Pointer(&newState))); err != 0 {
		return err
	}
	return nil
}

func HandleInterrupt(fd uintptr, out io.Writer, state *State) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	go func() {
		_ = <-sigchan
		fmt.Fprint(out, "\n")
		RestoreTerminal(fd, state)
		os.Exit(0)
	}()
}

func SetRawTerminal(fd uintptr, out io.Writer) (*State, error) {
	oldState, err := MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	HandleInterrupt(fd, out, oldState)
	return oldState, err
}
