package beam

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

type UnixConn struct {
	u *net.UnixConn
}

func NewUnixConn(u *net.UnixConn) *UnixConn {
	return &UnixConn{u}
}

func (conn *UnixConn) Send(msg Message) (err error) {
	var fds []int
	if msg.Stream != nil {
		f, err := msg.Stream.File()
		if err != nil {
			return fmt.Errorf("can't get fd from stream: %v", err)
		}
		fds = append(fds, int(f.Fd()))
	}
	_, _, err = conn.u.WriteMsgUnix(msg.Data, syscall.UnixRights(fds...), nil)
	return
}

func (conn *UnixConn) Receive() (msg Message, err error) {
	buf := make([]byte, 4096)
	oob := make([]byte, 4096)
	bufn, oobn, _, _, err := conn.u.ReadMsgUnix(buf, oob)
	if err != nil {
		err = fmt.Errorf("readmsg: %v", err)
		return
	}
	msg.Data = buf[:bufn]
	if fds := extractFds(oob[:oobn]); len(fds) > 0 {
		fd := uintptr(fds[0])
		msg.Stream = &File{os.NewFile(fd, fmt.Sprintf("%d", fd))}
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
