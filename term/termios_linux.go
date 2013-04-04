package term

import (
	"syscall"
	"unsafe"
)

// #include <termios.h>
// #include <sys/ioctl.h>
/*
void MakeRaw(int fd) {
  struct termios t;

  // FIXME: Handle errors?
  ioctl(fd, TCGETS, &t);

  t.c_iflag &= ~(IGNBRK | BRKINT | PARMRK | ISTRIP | INLCR | IGNCR | ICRNL | IXON);
  t.c_oflag &= ~OPOST;
  t.c_lflag &= ~(ECHO | ECHONL | ICANON | IEXTEN | ISIG);
  t.c_cflag &= ~(CSIZE | PARENB);
  t.c_cflag |= CS8;

  ioctl(fd, TCSETS, &t);
}
*/
import "C"

const (
	getTermios = syscall.TCGETS
	setTermios = syscall.TCSETS
)

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd int) (*State, error) {
	var oldState State
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), syscall.TCGETS, uintptr(unsafe.Pointer(&oldState.termios)), 0, 0, 0); err != 0 {
		return nil, err
	}
	C.MakeRaw(C.int(fd))
	return &oldState, nil

	// FIXME: post on goland issues this: very same as the C function bug non-working

	// newState := oldState.termios

	// newState.Iflag &^= (IGNBRK | BRKINT | PARMRK | ISTRIP | INLCR | IGNCR | ICRNL | IXON)
	// newState.Oflag &^= OPOST
	// newState.Lflag &^= (ECHO | syscall.ECHONL | ICANON | ISIG | IEXTEN)
	// newState.Cflag &^= (CSIZE | syscall.PARENB)
	// newState.Cflag |= CS8

	// if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TCSETS, uintptr(unsafe.Pointer(&newState))); err != 0 {
	// 	return nil, err
	// }
	// return &oldState, nil
}
