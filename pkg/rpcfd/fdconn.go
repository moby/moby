package rpcfd

import (
	"fmt"
	"net"
	"syscall"
)

type FdConn struct {
	conn           *net.UnixConn
	readFds        map[int]int
	readFdCount    int
	readCreds      map[int]*syscall.Ucred
	readCredsCount int

	writeFds        []int
	writeFdCount    int
	writeCreds      []*syscall.Ucred
	writeCredsCount int
}

func NewFdConn(conn *net.UnixConn) *FdConn {
	return &FdConn{
		conn:      conn,
		readFds:   make(map[int]int),
		readCreds: make(map[int]*syscall.Ucred),
	}
}

func (c *FdConn) Close() error {
	return c.conn.Close()
}

func (c *FdConn) Read(b []byte) (int, error) {
	oob := make([]byte, 1024)
	n, oobn, _, _, err := c.conn.ReadMsgUnix(b, oob)
	if err != nil {
		return 0, err
	}
	if oobn > 0 {
		messages, err := syscall.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			return n, err
		}
		for _, m := range messages {
			if m.Header.Level == syscall.SOL_SOCKET && m.Header.Type == syscall.SCM_RIGHTS {
				fds, err := syscall.ParseUnixRights(&m)
				if err != nil {
					return n, err
				}

				// Set the CLOEXEC flag on the FDs so they won't be leaked into future forks
				for _, fd := range fds {
					if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_SETFD, syscall.FD_CLOEXEC); errno != 0 {
						return n, errno
					}

					c.readFds[c.readFdCount] = fd
					c.readFdCount++
				}
			} else if m.Header.Level == syscall.SOL_SOCKET && m.Header.Type == syscall.SCM_CREDENTIALS {
				creds, err := syscall.ParseUnixCredentials(&m)
				if err != nil {
					return n, err
				}
				c.readCreds[c.readCredsCount] = creds
				c.readCredsCount++
			}

		}
	}

	return n, nil
}

func (c *FdConn) Write(b []byte) (int, error) {
	if len(c.writeFds) == 0 && len(c.writeCreds) == 0 {
		return c.conn.Write(b)
	} else {
		oob := []byte{}
		if len(c.writeFds) > 0 {
			oob = append(oob, syscall.UnixRights(c.writeFds...)...)
		}

		for _, creds := range c.writeCreds {
			oob = append(oob, syscall.UnixCredentials(creds)...)
		}

		n, _, err := c.conn.WriteMsgUnix(b, oob, nil)
		if err != nil {
			return 0, err
		}
		c.writeFds = nil
		c.writeCreds = nil
		return n, nil
	}
}

func (c *FdConn) AddWriteFd(fd int) int {
	c.writeFds = append(c.writeFds, fd)
	res := c.writeFdCount
	c.writeFdCount++
	return res
}

func (c *FdConn) AddWriteCreds(creds *syscall.Ucred) int {
	c.writeCreds = append(c.writeCreds, creds)
	res := c.writeCredsCount
	c.writeCredsCount++
	return res
}

func (c *FdConn) GetReadFd(index int) (int, error) {
	fd, ok := c.readFds[index]
	if !ok {
		return -1, fmt.Errorf("No received FD with index %d\n", index)
	}
	delete(c.readFds, index)
	return fd, nil
}

func (c *FdConn) GetReadCreds(index int) (*syscall.Ucred, error) {
	creds, ok := c.readCreds[index]
	if !ok {
		return nil, fmt.Errorf("No received creds with index %d\n", index)
	}
	delete(c.readCreds, index)
	return creds, nil
}
