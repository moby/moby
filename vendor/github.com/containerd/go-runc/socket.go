//go:build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Socket is a unix socket that accepts a file descriptor sent by runc, such as
// the pty master for the container's console or the pidfd of the process runc
// creates.
type Socket struct {
	rmdir bool
	l     *net.UnixListener
}

// newSocket creates a new unix socket listening at the provided path.
func newSocket(path string) (*Socket, error) {
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
		l: l,
	}, nil
}

// newTempSocket creates a unix socket named name inside a new temp directory
// created with the provided prefix. The directory is removed on Close().
func newTempSocket(prefix, name string) (*Socket, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	dir, err := os.MkdirTemp(runtimeDir, prefix)
	if err != nil {
		return nil, err
	}
	s, err := newSocket(filepath.Join(dir, name))
	if err != nil {
		_ = os.RemoveAll(dir) // #nosec G703 -- dir was created by os.MkdirTemp above.
		return nil, err
	}
	s.rmdir = true
	if runtimeDir != "" {
		if err := os.Chmod(s.Path(), 0o755|os.ModeSticky); err != nil { // #nosec G703 -- the path identifies the socket created in the temporary directory.
			_ = s.Close()
			return nil, err
		}
	}
	return s, nil
}

// Path returns the path to the unix socket on disk
func (c *Socket) Path() string {
	return c.l.Addr().String()
}

// Close closes the unix socket
func (c *Socket) Close() error {
	err := c.l.Close()
	if c.rmdir {
		if rErr := os.RemoveAll(filepath.Dir(c.Path())); err == nil { // #nosec G703 -- rmdir is set only for sockets created in a private temporary directory.
			err = rErr
		}
	}
	return err
}

// receive blocks until the socket receives a file descriptor
func (c *Socket) receive() (*os.File, error) {
	conn, err := c.l.Accept()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("received connection which was not a unix socket")
	}
	return recvFd(uc)
}

// recvFd waits for a file descriptor to be sent over the given AF_UNIX
// socket. The file name of the remote file descriptor will be recreated
// locally (it is sent as non-auxiliary data in the same payload).
func recvFd(socket *net.UnixConn) (*os.File, error) {
	const MaxNameLen = 4096
	oobSpace := unix.CmsgSpace(4)

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
