package client

import (
    "testing"
    "fmt"
)

const (
    serverProtocol = "unix"
    serverAddr = "/var/run/docker.sock"
)

func TestSomething(t *testing.T) {
    cl := NewClient(serverProtocol, serverAddr)
    containers, err := cl.ContainerList(true, true, 0, "","")

    if err != nil {
        t.Error(err)
    }

    fmt.Printf("%#v %#v", containers, err)
}
