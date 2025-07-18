package unix_noeintr

import (
	"errors"

	"golang.org/x/sys/unix"
)

func EpollCreate() (int, error) {
	for {
		fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return fd, err
	}
}

func EpollCtl(epFd int, op int, fd int, event *unix.EpollEvent) error {
	for {
		err := unix.EpollCtl(epFd, op, fd, event)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return err
	}
}

func EpollWait(epFd int, events []unix.EpollEvent, msec int) (int, error) {
	for {
		n, err := unix.EpollWait(epFd, events, msec)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return n, err
	}
}
