//go:build (linux || darwin) && !tinygo

package sysfs

import (
	"net"
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
)

// MSG_PEEK is the constant syscall.MSG_PEEK
const MSG_PEEK = syscall.MSG_PEEK

func newTCPListenerFile(tl *net.TCPListener) socketapi.TCPSock {
	return newDefaultTCPListenerFile(tl)
}

func _pollSock(conn syscall.Conn, flag fsapi.Pflag, timeoutMillis int32) (bool, sys.Errno) {
	n, errno := syscallConnControl(conn, func(fd uintptr) (int, sys.Errno) {
		if ready, errno := poll(fd, fsapi.POLLIN, 0); !ready || errno != 0 {
			return -1, errno
		} else {
			return 0, errno
		}
	})
	return n >= 0, errno
}

func setNonblockSocket(fd uintptr, enabled bool) sys.Errno {
	return sys.UnwrapOSError(setNonblock(fd, enabled))
}

func readSocket(fd uintptr, buf []byte) (int, sys.Errno) {
	n, err := syscall.Read(int(fd), buf)
	return n, sys.UnwrapOSError(err)
}

func writeSocket(fd uintptr, buf []byte) (int, sys.Errno) {
	n, err := syscall.Write(int(fd), buf)
	return n, sys.UnwrapOSError(err)
}

func recvfrom(fd uintptr, buf []byte, flags int32) (n int, errno sys.Errno) {
	n, _, err := syscall.Recvfrom(int(fd), buf, int(flags))
	return n, sys.UnwrapOSError(err)
}
