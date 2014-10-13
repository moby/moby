// +build ignore
package main

import (
	"fmt"
	"github.com/docker/docker/pkg/rpcfd"
	"log"
	"net"
	"net/rpc"
	"os"
	"syscall"
)

type RpcObject struct {
}

func (o *RpcObject) GetStdOut(a int, b *rpcfd.RpcFd) error {
	fmt.Printf("GetStdOut %d\n", a)
	b.Fd = 1
	return nil
}

func (o *RpcObject) GetPid(a int, b *rpcfd.RpcPid) error {
	fmt.Printf("GetPid %d\n", a)
	b.Pid = uintptr(syscall.Getpid())
	return nil
}

func (o *RpcObject) WriteToFd(a rpcfd.RpcFd, b *int) error {
	fmt.Printf("WriteToFd %v\n", a)
	syscall.Write(int(a.Fd), []byte("Hello from server\n"))
	return nil
}

func main() {
	object := &RpcObject{}

	if err := rpc.Register(object); err != nil {
		log.Fatal(err)
	}

	os.Remove("/tmp/test.socket")
	addr := &net.UnixAddr{Net: "unix", Name: "/tmp/test.socket"}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			log.Printf("rpc socket accept error: %s", err)
			continue
		}

		fmt.Printf("New client connected\n")
		rpcfd.ServeConn(conn)
		conn.Close()
	}
}
