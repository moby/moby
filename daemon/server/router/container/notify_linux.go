package container

import (
	"context"
	"net"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/unix_noeintr"
	"golang.org/x/sys/unix"
)

func notifyClosed(ctx context.Context, conn net.Conn, notify func()) {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		log.G(ctx).Debug("notifyClosed: conn does not support close notifications")
		return
	}

	rc, err := sc.SyscallConn()
	if err != nil {
		log.G(ctx).WithError(err).Warn("notifyClosed: failed get raw conn for close notifications")
		return
	}

	epFd, err := unix_noeintr.EpollCreate()
	if err != nil {
		log.G(ctx).WithError(err).Warn("notifyClosed: failed to create epoll fd")
		return
	}
	defer unix.Close(epFd)

	err = rc.Control(func(fd uintptr) {
		err := unix_noeintr.EpollCtl(epFd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{
			Events: unix.EPOLLHUP,
			Fd:     int32(fd),
		})
		if err != nil {
			log.G(ctx).WithError(err).Warn("notifyClosed: failed to register fd for close notifications")
			return
		}

		events := make([]unix.EpollEvent, 1)
		if _, err := unix_noeintr.EpollWait(epFd, events, -1); err != nil {
			log.G(ctx).WithError(err).Warn("notifyClosed: failed to wait for close notifications")
			return
		}
		notify()
	})
	if err != nil {
		log.G(ctx).WithError(err).Warn("notifyClosed: failed to register for close notifications")
		return
	}
}
