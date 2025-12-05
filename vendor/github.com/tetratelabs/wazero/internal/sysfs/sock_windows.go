//go:build windows

package sysfs

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
)

const (
	// MSG_PEEK is the flag PEEK for syscall.Recvfrom on Windows.
	// This constant is not exported on this platform.
	MSG_PEEK = 0x2
	// _FIONBIO is the flag to set the O_NONBLOCK flag on socket handles using ioctlsocket.
	_FIONBIO = 0x8004667e
)

var (
	// modws2_32 is WinSock.
	modws2_32 = syscall.NewLazyDLL("ws2_32.dll")
	// procrecvfrom exposes recvfrom from WinSock.
	procrecvfrom = modws2_32.NewProc("recvfrom")
	// procioctlsocket exposes ioctlsocket from WinSock.
	procioctlsocket = modws2_32.NewProc("ioctlsocket")
)

func newTCPListenerFile(tl *net.TCPListener) socketapi.TCPSock {
	return newDefaultTCPListenerFile(tl)
}

// recvfrom exposes the underlying syscall in Windows.
//
// Note: since we are only using this to expose MSG_PEEK,
// we do not need really need all the parameters that are actually
// allowed in WinSock.
// We ignore `from *sockaddr` and `fromlen *int`.
func recvfrom(s uintptr, buf []byte, flags int32) (n int, errno sys.Errno) {
	var _p0 *byte
	if len(buf) > 0 {
		_p0 = &buf[0]
	}
	r0, _, e1 := syscall.SyscallN(
		procrecvfrom.Addr(),
		s,
		uintptr(unsafe.Pointer(_p0)),
		uintptr(len(buf)),
		uintptr(flags),
		0, // from *sockaddr (optional)
		0) // fromlen *int (optional)
	return int(r0), sys.UnwrapOSError(e1)
}

func setNonblockSocket(fd uintptr, enabled bool) sys.Errno {
	opt := uint64(0)
	if enabled {
		opt = 1
	}
	// ioctlsocket(fd, FIONBIO, &opt)
	_, _, errno := syscall.SyscallN(
		procioctlsocket.Addr(),
		uintptr(fd),
		uintptr(_FIONBIO),
		uintptr(unsafe.Pointer(&opt)))
	return sys.UnwrapOSError(errno)
}

func _pollSock(conn syscall.Conn, flag fsapi.Pflag, timeoutMillis int32) (bool, sys.Errno) {
	if flag != fsapi.POLLIN {
		return false, sys.ENOTSUP
	}
	n, errno := syscallConnControl(conn, func(fd uintptr) (int, sys.Errno) {
		return _poll([]pollFd{newPollFd(fd, _POLLIN, 0)}, timeoutMillis)
	})
	return n > 0, errno
}
