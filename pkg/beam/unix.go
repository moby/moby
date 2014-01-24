package beam

import (
	"os"
	"fmt"
	"net"
	"syscall"
)

type UnixConn struct {
	u *net.UnixConn
}

func (conn *UnixConn) Send(data []byte, s Stream) (err error) {
	var fds []int
	if s != nil {
		f, err := s.File()
		if err != nil {
			return fmt.Errorf("can't get fd from stream: %s", err)
		}
		fds = append(fds, int(f.Fd()))
	}
	_, _, err = conn.u.WriteMsgUnix(data, syscall.UnixRights(fds...), nil)
	return err
}

func (conn *UnixConn) Receive() (data []byte, s Stream, err error) {
	buf := make([]byte, 4096)
	oob := make([]byte, 4096)
	bufn, oobn, _, _, err := conn.u.ReadMsgUnix(buf, oob)
	if err != nil {
		return nil, nil, fmt.Errorf("readmsg: %s", err)
	}
	data = buf[:bufn]
	if fds := extractFds(oob[:oobn]); len(fds) > 0 {
		fd := uintptr(fds[0])
		s = &File{os.NewFile(fd, fmt.Sprintf("%d", fd))}
	}
	return
}

func (conn *UnixConn) File() (*os.File, error) {
	return conn.u.File()
}

// Close a stream.
// Closing a stream must *not* cause the closing of streams sent through it.
func (conn *UnixConn) Close() error {
	return conn.u.Close()
}

func extractFds(oob []byte) (fds []int) {
	scms, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return
	}
	for _, scm := range scms {
		gotFds, err := syscall.ParseUnixRights(&scm)
		if err != nil {
			continue
		}
		fds = append(fds, gotFds...)
	}
	return
}
