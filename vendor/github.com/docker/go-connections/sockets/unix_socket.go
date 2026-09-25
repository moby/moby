/*
Package sockets is a simple unix domain socket wrapper.

# Usage

For example:

	import(
		"fmt"
		"net"
		"os"
		"github.com/docker/go-connections/sockets"
	)

	func main() {
		l, err := sockets.NewUnixSocketWithOpts("/path/to/sockets",
			sockets.WithChown(0,0),sockets.WithChmod(0660))
		if err != nil {
			panic(err)
		}
		echoStr := "hello"

		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				conn.Write([]byte(echoStr))
				conn.Close()
			}
		}()

		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 5)
		if _, err := conn.Read(buf); err != nil {
			panic(err)
		} else if string(buf) != echoStr {
			panic(fmt.Errorf("msg may lost"))
		}
	}
*/
package sockets

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
)

const supportsAbstractSockets = runtime.GOOS == "linux"

// SockOption sets up socket file's creating option
type SockOption func(string) error

// NewUnixSocketWithOpts creates a Unix socket with the specified options.
//
// On Unix platforms, socket permissions are 0000 by default, i.e. no access
// for anyone. Pass WithChmod() and WithChown() to set the desired permissions
// and ownership.
//
// On Windows, the socket uses Windows ACLs. Pass WithBasePermissions() to allow
// Administrators and LocalSystem full access, or WithAdditionalUsersAndGroups()
// to also grant generic read and write access to additional users or groups.
//
// Abstract Unix sockets (Go's Linux-specific "@" shorthand and the native
// leading-NUL representation) are supported only on Linux. On other platforms,
// attempts to use abstract socket addresses return an error. Because abstract
// sockets have no filesystem representation, filesystem-specific socket
// options are not supported.
//
// On platforms without abstract Unix socket support, attempts to use abstract
// socket addresses return an error wrapping [errors.ErrUnsupported].
func NewUnixSocketWithOpts(path string, opts ...SockOption) (net.Listener, error) {
	if isAbstractSocket(path) {
		if !supportsAbstractSockets {
			return nil, fmt.Errorf("abstract Unix socket %q is not supported on %s: %w", path, runtime.GOOS, errors.ErrUnsupported)
		}
		for _, opt := range opts {
			if err := opt(path); err != nil {
				return nil, err
			}
		}
		return net.Listen("unix", path)
	}
	if err := syscall.Unlink(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return listenUnix(path, opts...)
}

// isAbstractSocket reports whether path is an abstract Unix socket address.
//
// Go recognizes two representations of abstract socket addresses:
//
//   - On Linux, a path beginning with '@' is translated by the standard library
//     to the kernel's native leading-NUL representation.
//     See https://pkg.go.dev/net@go1.27rc2#UnixAddr.
//
//   - A path beginning with a NUL byte uses the kernel's native representation
//     directly. See https://github.com/golang/go/issues/78615.
//
// The interpretation of these addresses is platform-dependent; this helper only
// recognizes the syntax.
func isAbstractSocket(path string) bool {
	return len(path) > 0 && (path[0] == '@' || path[0] == 0)
}
