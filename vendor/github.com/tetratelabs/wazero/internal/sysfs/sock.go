package sysfs

import (
	"net"
	"os"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
	"github.com/tetratelabs/wazero/sys"
)

// NewTCPListenerFile creates a socketapi.TCPSock for a given *net.TCPListener.
func NewTCPListenerFile(tl *net.TCPListener) socketapi.TCPSock {
	return newTCPListenerFile(tl)
}

// baseSockFile implements base behavior for all TCPSock, TCPConn files,
// regardless the platform.
type baseSockFile struct {
	experimentalsys.UnimplementedFile
}

var _ experimentalsys.File = (*baseSockFile)(nil)

// IsDir implements the same method as documented on File.IsDir
func (*baseSockFile) IsDir() (bool, experimentalsys.Errno) {
	// We need to override this method because WASI-libc prestats the FD
	// and the default impl returns ENOSYS otherwise.
	return false, 0
}

// Stat implements the same method as documented on File.Stat
func (f *baseSockFile) Stat() (fs sys.Stat_t, errno experimentalsys.Errno) {
	// The mode is not really important, but it should be neither a regular file nor a directory.
	fs.Mode = os.ModeIrregular
	return
}

var _ socketapi.TCPSock = (*tcpListenerFile)(nil)

type tcpListenerFile struct {
	baseSockFile

	tl       *net.TCPListener
	closed   bool
	nonblock bool
}

// newTCPListenerFile is a constructor for a socketapi.TCPSock.
//
// The current strategy is to wrap a net.TCPListener
// and invoking raw syscalls using syscallConnControl:
// this internal calls RawConn.Control(func(fd)), making sure
// that the underlying file descriptor is valid throughout
// the duration of the syscall.
func newDefaultTCPListenerFile(tl *net.TCPListener) socketapi.TCPSock {
	return &tcpListenerFile{tl: tl}
}

// Close implements the same method as documented on experimentalsys.File
func (f *tcpListenerFile) Close() experimentalsys.Errno {
	if !f.closed {
		return experimentalsys.UnwrapOSError(f.tl.Close())
	}
	return 0
}

// Addr is exposed for testing.
func (f *tcpListenerFile) Addr() *net.TCPAddr {
	return f.tl.Addr().(*net.TCPAddr)
}

// IsNonblock implements the same method as documented on fsapi.File
func (f *tcpListenerFile) IsNonblock() bool {
	return f.nonblock
}

// Poll implements the same method as documented on fsapi.File
func (f *tcpListenerFile) Poll(flag fsapi.Pflag, timeoutMillis int32) (ready bool, errno experimentalsys.Errno) {
	return false, experimentalsys.ENOSYS
}

var _ socketapi.TCPConn = (*tcpConnFile)(nil)

type tcpConnFile struct {
	baseSockFile

	tc *net.TCPConn

	// nonblock is true when the underlying connection is flagged as non-blocking.
	// This ensures that reads and writes return experimentalsys.EAGAIN without blocking the caller.
	nonblock bool
	// closed is true when closed was called. This ensures proper experimentalsys.EBADF
	closed bool
}

func newTcpConn(tc *net.TCPConn) socketapi.TCPConn {
	return &tcpConnFile{tc: tc}
}

// Read implements the same method as documented on experimentalsys.File
func (f *tcpConnFile) Read(buf []byte) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // Short-circuit 0-len reads.
	}
	if nonBlockingFileReadSupported && f.IsNonblock() {
		n, errno = syscallConnControl(f.tc, func(fd uintptr) (int, experimentalsys.Errno) {
			n, err := readSocket(fd, buf)
			errno = experimentalsys.UnwrapOSError(err)
			errno = fileError(f, f.closed, errno)
			return n, errno
		})
	} else {
		n, errno = read(f.tc, buf)
	}
	if errno != 0 {
		// Defer validation overhead until we've already had an error.
		errno = fileError(f, f.closed, errno)
	}
	return
}

// Write implements the same method as documented on experimentalsys.File
func (f *tcpConnFile) Write(buf []byte) (n int, errno experimentalsys.Errno) {
	if nonBlockingFileWriteSupported && f.IsNonblock() {
		return syscallConnControl(f.tc, func(fd uintptr) (int, experimentalsys.Errno) {
			n, err := writeSocket(fd, buf)
			errno = experimentalsys.UnwrapOSError(err)
			errno = fileError(f, f.closed, errno)
			return n, errno
		})
	} else {
		n, errno = write(f.tc, buf)
	}
	if errno != 0 {
		// Defer validation overhead until we've already had an error.
		errno = fileError(f, f.closed, errno)
	}
	return
}

// Recvfrom implements the same method as documented on socketapi.TCPConn
func (f *tcpConnFile) Recvfrom(p []byte, flags int) (n int, errno experimentalsys.Errno) {
	if flags != MSG_PEEK {
		errno = experimentalsys.EINVAL
		return
	}
	return syscallConnControl(f.tc, func(fd uintptr) (int, experimentalsys.Errno) {
		n, err := recvfrom(fd, p, MSG_PEEK)
		errno = experimentalsys.UnwrapOSError(err)
		errno = fileError(f, f.closed, errno)
		return n, errno
	})
}

// Close implements the same method as documented on experimentalsys.File
func (f *tcpConnFile) Close() experimentalsys.Errno {
	return f.close()
}

func (f *tcpConnFile) close() experimentalsys.Errno {
	if f.closed {
		return 0
	}
	f.closed = true
	return f.Shutdown(socketapi.SHUT_RDWR)
}

// SetNonblock implements the same method as documented on fsapi.File
func (f *tcpConnFile) SetNonblock(enabled bool) (errno experimentalsys.Errno) {
	f.nonblock = enabled
	_, errno = syscallConnControl(f.tc, func(fd uintptr) (int, experimentalsys.Errno) {
		return 0, experimentalsys.UnwrapOSError(setNonblockSocket(fd, enabled))
	})
	return
}

// IsNonblock implements the same method as documented on fsapi.File
func (f *tcpConnFile) IsNonblock() bool {
	return f.nonblock
}

// Poll implements the same method as documented on fsapi.File
func (f *tcpConnFile) Poll(flag fsapi.Pflag, timeoutMillis int32) (ready bool, errno experimentalsys.Errno) {
	return false, experimentalsys.ENOSYS
}
