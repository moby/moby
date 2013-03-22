package term

import (
    "syscall"
    "unsafe"
)

const (
	getTermios = syscall.TCGETS
	setTermios = syscall.TCSETS
)

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd int) (*State, error) {
    var oldState State
    if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios)), 0, 0, 0); err != 0 {
        return nil, err
    }

    newState := oldState.termios
    newState.Iflag &^= ISTRIP | IXON | IXOFF
    newState.Iflag |= ICRNL
    newState.Oflag |= ONLCR
    newState.Lflag &^= ECHO | ICANON | ISIG
    if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
        return nil, err
    }

    return &oldState, nil
}