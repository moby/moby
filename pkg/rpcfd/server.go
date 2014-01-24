package rpcfd

import (
	"bufio"
	"encoding/gob"
	"net"
	"net/rpc"
	"syscall"
)

type gobServerCodec struct {
	fdConn *FdConn
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
}

func (c *gobServerCodec) ReadRequestHeader(r *rpc.Request) error {
	return c.dec.Decode(r)
}

func (c *gobServerCodec) ReadRequestBody(body interface{}) error {
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

func (c *gobServerCodec) WriteResponse(r *rpc.Response, body interface{}) (err error) {
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

func (c *gobServerCodec) Close() error {
	return c.fdConn.Close()
}

func ServeConn(conn *net.UnixConn) {
	fdConn := NewFdConn(conn)

	buf := bufio.NewWriter(fdConn)
	srv := &gobServerCodec{fdConn, gob.NewDecoder(fdConn), gob.NewEncoder(buf), buf}
	rpc.ServeCodec(srv)
}
