package main

import (
	"fmt"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/pkg/beam"
	"net"
	"os"
)

func main() {
	eng := engine.New()

	c, err := net.Dial("unix", "beam.sock")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	defer c.Close()
	f, err := c.(*net.UnixConn).File()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	child, err := beam.FileConn(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	defer child.Close()

	sender := engine.NewSender(child)
	sender.Install(eng)

	cmd := eng.Job(os.Args[1], os.Args[2:]...)
	cmd.Stdout.Add(os.Stdout)
	cmd.Stderr.Add(os.Stderr)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
