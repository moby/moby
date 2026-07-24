//go:build !windows

package sockets

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
)

// defaultSocketPerms is the default permission mode applied to newly created
// Unix sockets. Sockets are created inaccessible by default; callers can
// override this by passing [WithChmod].
//
// TODO(thaJeztah): Consider changing the default to 0o600, making the socket usable by its owner by default.
const defaultSocketPerms os.FileMode = 0o000

// WithChown modifies the socket file's uid and gid.
//
// Abstract Unix sockets have no filesystem representation, so this option
// returns an error wrapping [errors.ErrUnsupported] when used with an abstract
// socket.
func WithChown(uid, gid int) SockOption {
	return func(path string) error {
		if isAbstractSocket(path) {
			return &os.PathError{
				Op:   "chown",
				Path: path,
				Err:  fmt.Errorf("abstract Unix sockets do not support filesystem permissions: %w", errors.ErrUnsupported),
			}
		}
		if err := os.Chown(path, uid, gid); err != nil {
			return err
		}
		return nil
	}
}

// WithChmod modifies socket file's access mode.
//
// Abstract Unix sockets have no filesystem representation, so this option
// returns an error wrapping [errors.ErrUnsupported] when used with an abstract
// socket.
func WithChmod(mask os.FileMode) SockOption {
	return func(path string) error {
		if isAbstractSocket(path) {
			return &os.PathError{
				Op:   "chmod",
				Path: path,
				Err:  fmt.Errorf("abstract Unix sockets do not support filesystem permissions: %w", errors.ErrUnsupported),
			}
		}
		if err := os.Chmod(path, mask); err != nil {
			return err
		}
		return nil
	}
}

// NewUnixSocket creates a Unix socket with the specified path and group.
//
// On Unix platforms, the socket is owned by root:gid and has permissions 0660.
//
// Abstract Unix sockets are not supported by this helper. Use [NewUnixSocketWithOpts]
// without filesystem permission options instead.
func NewUnixSocket(path string, gid int) (net.Listener, error) {
	return NewUnixSocketWithOpts(path, WithChown(0, gid), WithChmod(0o660))
}

func listenUnix(path string, opts ...SockOption) (_ net.Listener, retErr error) {
	// net.Listen does not allow permissions or ownership to be set between
	// bind(2), which creates the socket path, and listen(2), which makes it
	// possible for clients to connect.
	//
	// Creating the socket manually lets us apply options after bind(2), but
	// before listen(2). This avoids temporarily relaxing the process umask while
	// still preventing a socket from becoming connectable before the requested
	// permissions are applied.
	//
	// See https://github.com/golang/go/issues/11822

	// Similar to sysSocket in stdlib, but without the fast path for Linux.
	// https://github.com/golang/go/blob/go1.26.3/src/net/sys_cloexec.go#L18-L36
	syscall.ForkLock.RLock()
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err == nil {
		syscall.CloseOnExec(fd) // No syscall.SOCK_CLOEXEC on macOS.
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}

	defer func() {
		if fd >= 0 {
			_ = syscall.Close(fd)
		}
	}()

	if err := syscall.Bind(fd, &syscall.SockaddrUnix{Name: path}); err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	defer func() {
		if retErr != nil {
			_ = syscall.Unlink(path)
		}
	}()

	// Secure by default: the socket is not accessible at all
	// unless permission options are set through WithChmod.
	if err := os.Chmod(path, defaultSocketPerms); err != nil {
		return nil, err
	}

	for _, op := range opts {
		if err := op(path); err != nil {
			return nil, err
		}
	}

	if err := syscall.Listen(fd, listenerBacklog()); err != nil {
		return nil, os.NewSyscallError("listen", err)
	}

	f := os.NewFile(uintptr(fd), "unix:"+path)
	fd = -1 // f now owns the original fd; prevent the defer from closing it.

	// FileListener duplicates f, sets the duplicate close-on-exec and nonblocking,
	// and returns a net.Listener backed by that duplicate. The temporary *os.File
	// is no longer needed after this point.
	l, err := net.FileListener(f)
	_ = f.Close()
	if err != nil {
		return nil, err
	}

	if ul, ok := l.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(true)
	}

	return l, nil
}

// listenerBacklog is a caching wrapper around maxListenerBacklog.
var listenerBacklog = sync.OnceValue(maxListenerBacklog)
