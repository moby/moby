package runc

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/containerd/console"
	"golang.org/x/sys/unix"
)

// NewConsoleSocket creates a new unix socket at the provided path to accept a
// pty master created by runc for use by the container
func NewConsoleSocket(path string) (*Socket, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	addr, err := net.ResolveUnixAddr("unix", abs)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}
	return &Socket{
		l:    l,
	}, nil
}

// NewTempConsoleSocket returns a temp console socket for use with a container
// On Close(), the socket is deleted
func NewTempConsoleSocket() (*Socket, error) {
	dir, err := ioutil.TempDir("", "pty")
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(filepath.Join(dir, "pty.sock"))
	if err != nil {
		return nil, err
	}
	addr, err := net.ResolveUnixAddr("unix", abs)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}
	return &Socket{
		l:     l,
		rmdir: true,
	}, nil
}

// Socket is a unix socket that accepts the pty master created by runc
type Socket struct {
	rmdir bool
	l     *net.UnixListener
}

// Path returns the path to the unix socket on disk
func (c *Socket) Path() string {
	return c.l.Addr().String()
}

// recvFd waits for a file descriptor to be sent over the given AF_UNIX
// socket. The file name of the remote file descriptor will be recreated
// locally (it is sent as non-auxiliary data in the same payload).
func recvFd(socket *net.UnixConn) (*os.File, error) {
	const MaxNameLen = 4096
	var oobSpace = unix.CmsgSpace(4)

	name := make([]byte, MaxNameLen)
	oob := make([]byte, oobSpace)

	n, oobn, _, _, err := socket.ReadMsgUnix(name, oob)
	if err != nil {
		return nil, err
	}

	if n >= MaxNameLen || oobn != oobSpace {
		return nil, fmt.Errorf("recvfd: incorrect number of bytes read (n=%d oobn=%d)", n, oobn)
	}

	// Truncate.
	name = name[:n]
	oob = oob[:oobn]

	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, err
	}
	if len(scms) != 1 {
		return nil, fmt.Errorf("recvfd: number of SCMs is not 1: %d", len(scms))
	}
	scm := scms[0]

	fds, err := unix.ParseUnixRights(&scm)
	if err != nil {
		return nil, err
	}
	if len(fds) != 1 {
		return nil, fmt.Errorf("recvfd: number of fds is not 1: %d", len(fds))
	}
	fd := uintptr(fds[0])

	return os.NewFile(fd, string(name)), nil
}

// ReceiveMaster blocks until the socket receives the pty master
func (c *Socket) ReceiveMaster() (console.Console, error) {
	conn, err := c.l.Accept()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("received connection which was not a unix socket")
	}
	f, err := recvFd(uc)
	if err != nil {
		return nil, err
	}
	return console.ConsoleFromFile(f)
}

// Close closes the unix socket
func (c *Socket) Close() error {
	err := c.l.Close()
	if c.rmdir {
		if rerr := os.RemoveAll(filepath.Dir(c.Path())); err == nil {
			err = rerr
		}
	}
	return err
}
