// +build linux,gccgo

package term

import (
	"syscall"
)

const (
	getTermios = syscall.TCGETS
	setTermios = syscall.TCSETS
)

type Termios syscall.Termios

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	var oldState State
	if err := getTerminalState(fd, &oldState.termios); err != 0 {
		return nil, err
	}

	newState := oldState.termios

	newState.Iflag &^= (syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON)
	newState.Oflag &^= syscall.OPOST
	newState.Lflag &^= (syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN)
	newState.Cflag &^= (syscall.CSIZE | syscall.PARENB)
	newState.Cflag |= syscall.CS8

	if err := setTerminalState(fd, &newState); err != 0 {
		return nil, err
	}
	return &oldState, nil
}

func getTerminalState(fd uintptr, p *Termios) syscall.Errno {
	err := syscall.Tcgetattr(int(fd), (*syscall.Termios)(p))
	if err != nil {
		return syscall.GetErrno()
	}
	return 0
}

func setTerminalState(fd uintptr, p *Termios) syscall.Errno {
	err := syscall.Tcsetattr(int(fd), syscall.TCSANOW, (*syscall.Termios)(p))
	if err != nil {
		return syscall.GetErrno()
	}
	return 0
}
