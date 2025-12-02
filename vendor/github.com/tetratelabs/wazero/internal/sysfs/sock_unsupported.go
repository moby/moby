//go:build (!linux && !darwin && !windows) || tinygo

package sysfs

import (
	"net"
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
)

// MSG_PEEK is a filler value.
const MSG_PEEK = 0x2

func newTCPListenerFile(tl *net.TCPListener) socketapi.TCPSock {
	return &unsupportedSockFile{}
}

type unsupportedSockFile struct {
	baseSockFile
}

// Accept implements the same method as documented on socketapi.TCPSock
func (f *unsupportedSockFile) Accept() (socketapi.TCPConn, sys.Errno) {
	return nil, sys.ENOSYS
}

func _pollSock(conn syscall.Conn, flag fsapi.Pflag, timeoutMillis int32) (bool, sys.Errno) {
	return false, sys.ENOTSUP
}

func setNonblockSocket(fd uintptr, enabled bool) sys.Errno {
	return sys.ENOTSUP
}

func readSocket(fd uintptr, buf []byte) (int, sys.Errno) {
	return -1, sys.ENOTSUP
}

func writeSocket(fd uintptr, buf []byte) (int, sys.Errno) {
	return -1, sys.ENOTSUP
}

func recvfrom(fd uintptr, buf []byte, flags int32) (n int, errno sys.Errno) {
	return -1, sys.ENOTSUP
}

// syscallConnControl extracts a syscall.RawConn from the given syscall.Conn and applies
// the given fn to a file descriptor, returning an integer or a nonzero syscall.Errno on failure.
//
// syscallConnControl streamlines the pattern of extracting the syscall.Rawconn,
// invoking its syscall.RawConn.Control method, then handling properly the errors that may occur
// within fn or returned by syscall.RawConn.Control itself.
func syscallConnControl(conn syscall.Conn, fn func(fd uintptr) (int, experimentalsys.Errno)) (n int, errno sys.Errno) {
	return -1, sys.ENOTSUP
}

// Accept implements the same method as documented on socketapi.TCPSock
func (f *tcpListenerFile) Accept() (socketapi.TCPConn, experimentalsys.Errno) {
	return nil, experimentalsys.ENOSYS
}

// Shutdown implements the same method as documented on experimentalsys.Conn
func (f *tcpConnFile) Shutdown(how int) experimentalsys.Errno {
	// FIXME: can userland shutdown listeners?
	var err error
	switch how {
	case socketapi.SHUT_RD:
		err = f.tc.Close()
	case socketapi.SHUT_WR:
		err = f.tc.Close()
	case socketapi.SHUT_RDWR:
		return f.close()
	default:
		return experimentalsys.EINVAL
	}
	return experimentalsys.UnwrapOSError(err)
}
