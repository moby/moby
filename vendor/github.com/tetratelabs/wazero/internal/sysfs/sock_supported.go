//go:build (linux || darwin || windows) && !tinygo

package sysfs

import (
	"net"
	"syscall"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
)

// Accept implements the same method as documented on socketapi.TCPSock
func (f *tcpListenerFile) Accept() (socketapi.TCPConn, experimentalsys.Errno) {
	// Ensure we have an incoming connection, otherwise return immediately.
	if f.nonblock {
		if ready, errno := _pollSock(f.tl, fsapi.POLLIN, 0); !ready || errno != 0 {
			return nil, experimentalsys.EAGAIN
		}
	}

	// Accept normally blocks goroutines, but we
	// made sure that we have an incoming connection,
	// so we should be safe.
	if conn, err := f.tl.Accept(); err != nil {
		return nil, experimentalsys.UnwrapOSError(err)
	} else {
		return newTcpConn(conn.(*net.TCPConn)), 0
	}
}

// SetNonblock implements the same method as documented on fsapi.File
func (f *tcpListenerFile) SetNonblock(enabled bool) (errno experimentalsys.Errno) {
	f.nonblock = enabled
	_, errno = syscallConnControl(f.tl, func(fd uintptr) (int, experimentalsys.Errno) {
		return 0, setNonblockSocket(fd, enabled)
	})
	return
}

// Shutdown implements the same method as documented on experimentalsys.Conn
func (f *tcpConnFile) Shutdown(how int) experimentalsys.Errno {
	// FIXME: can userland shutdown listeners?
	var err error
	switch how {
	case socketapi.SHUT_RD:
		err = f.tc.CloseRead()
	case socketapi.SHUT_WR:
		err = f.tc.CloseWrite()
	case socketapi.SHUT_RDWR:
		return f.close()
	default:
		return experimentalsys.EINVAL
	}
	return experimentalsys.UnwrapOSError(err)
}

// syscallConnControl extracts a syscall.RawConn from the given syscall.Conn and applies
// the given fn to a file descriptor, returning an integer or a nonzero syscall.Errno on failure.
//
// syscallConnControl streamlines the pattern of extracting the syscall.Rawconn,
// invoking its syscall.RawConn.Control method, then handling properly the errors that may occur
// within fn or returned by syscall.RawConn.Control itself.
func syscallConnControl(conn syscall.Conn, fn func(fd uintptr) (int, experimentalsys.Errno)) (n int, errno experimentalsys.Errno) {
	syscallConn, err := conn.SyscallConn()
	if err != nil {
		return 0, experimentalsys.UnwrapOSError(err)
	}
	// Prioritize the inner errno over Control
	if controlErr := syscallConn.Control(func(fd uintptr) {
		n, errno = fn(fd)
	}); errno == 0 {
		errno = experimentalsys.UnwrapOSError(controlErr)
	}
	return
}
