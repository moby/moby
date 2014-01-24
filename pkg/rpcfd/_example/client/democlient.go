// +build ignore
package main

import (
	"fmt"
	"github.com/docker/docker/pkg/rpcfd"
	"net"
	"net/rpc"
	"syscall"
)

func main() {
	var dockerInitRpc *rpc.Client
	addr, err := net.ResolveUnixAddr("unix", "/tmp/test.socket")
	if err != nil {
		fmt.Printf("resolv: %v\n", err)
	}
	if socket, err := net.DialUnix("unix", nil, addr); err != nil {
		fmt.Printf("dial Error: %v\n", err)
		return
	} else {
		dockerInitRpc = rpcfd.NewClient(socket)
	}

	var arg int
	var ret rpcfd.RpcFd
	arg = 41
	if err := dockerInitRpc.Call("RpcObject.GetStdOut", &arg, &ret); err != nil {
		fmt.Printf("resume Error: %v\n", err)
		return
	}
	syscall.Write(int(ret.Fd), []byte("Hello from client 1\n"))

	// Call it again to test multiple calls

	arg = 42
	if err := dockerInitRpc.Call("RpcObject.GetStdOut", &arg, &ret); err != nil {
		fmt.Printf("resume Error: %v\n", err)
		return
	}
	syscall.Write(int(ret.Fd), []byte("Hello from client 2\n"))

	var fd rpcfd.RpcFd
	var dummy int
	if err := dockerInitRpc.Call("RpcObject.WriteToFd", &fd, &dummy); err != nil {
		fmt.Printf("resume Error: %v\n", err)
		return
	}
	if err := dockerInitRpc.Call("RpcObject.WriteToFd", &fd, &dummy); err != nil {
		fmt.Printf("resume Error: %v\n", err)
		return
	}

	var pid rpcfd.RpcPid
	if err := dockerInitRpc.Call("RpcObject.GetPid", &arg, &pid); err != nil {
		fmt.Printf("GetPid Error: %v\n", err)
		return
	}
	fmt.Printf("getpid result: %d\n", pid.Pid)
}
