package rpcfd

import (
	"bufio"
	"encoding/gob"
	"net"
	"net/rpc"
	"syscall"
)

type RpcFd struct {
	Fd uintptr
}

type RpcPid struct {
	Pid uintptr
}

type gobClientCodec struct {
	fdConn *FdConn
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
}

func (c *gobClientCodec) WriteRequest(r *rpc.Request, body interface{}) (err error) {
	if fd, ok := body.(*RpcFd); ok {
		fd.Fd = uintptr(c.fdConn.AddWriteFd(int(fd.Fd)))
	}

	if pid, ok := body.(*RpcPid); ok {
		creds := &syscall.Ucred{
			Pid: int32(pid.Pid),
			Uid: uint32(syscall.Getuid()),
			Gid: uint32(syscall.Getgid()),
		}
		pid.Pid = uintptr(c.fdConn.AddWriteCreds(creds))
	}

	if err = c.enc.Encode(r); err != nil {
		return
	}

	if err = c.enc.Encode(body); err != nil {
		return
	}
	return c.encBuf.Flush()
}

func (c *gobClientCodec) ReadResponseHeader(r *rpc.Response) error {
	return c.dec.Decode(r)
}

func (c *gobClientCodec) ReadResponseBody(body interface{}) error {
	if err := c.dec.Decode(body); err != nil {
		return err
	}
	if fd, ok := body.(*RpcFd); ok {
		index := int(fd.Fd)
		newFd, err := c.fdConn.GetReadFd(index)
		if err != nil {
			return err
		}
		fd.Fd = uintptr(newFd)
	}

	if pid, ok := body.(*RpcPid); ok {
		index := int(pid.Pid)
		creds, err := c.fdConn.GetReadCreds(index)
		if err != nil {
			return err
		}
		pid.Pid = uintptr(creds.Pid)
	}

	return nil
}

func (c *gobClientCodec) Close() error {
	return c.fdConn.Close()
}

func NewClient(conn *net.UnixConn) *rpc.Client {
	fdConn := NewFdConn(conn)

	fd, _ := conn.File()
	if fd != nil {
		syscall.SetsockoptInt(int(fd.Fd()), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
		fd.Close()
	}

	encBuf := bufio.NewWriter(fdConn)
	client := &gobClientCodec{fdConn, gob.NewDecoder(fdConn), gob.NewEncoder(encBuf), encBuf}
	return rpc.NewClientWithCodec(client)
}
