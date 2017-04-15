package main

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func main() {
	hub := beam.Hub()
	commands := Commands()
	cli := beam.Cli(os.Stdin, os.Stdout, os.Stderr)
	hub.Send(beam.Message{Data: []byte("register exec echo listen"), Stream: commands})
	beam.Copy(hub, cli)
}

func Commands() beam.Stream {
	inside, outside := beam.Pipe()
	go func() {
		defer inside.Close()
		for {
			msg, err := inside.Receive()
			if err != nil {
				return
			}
			parts := strings.Split(string(msg.Data), " ")
			if name := parts[0]; name == "echo" {
				go beam.JobHandler(Echo).Send(msg)
			} else if name == "exec" {
				go beam.JobHandler(Exec).Send(msg)
			} else if name == "listen" {
				go beam.JobHandler(Listen).Send(msg)
			} else {
				go beam.JobHandler(Unknown).Send(msg)
			}
		}
	}()
	return outside
}

func Echo(job *beam.Job) {
	fmt.Fprintf(job.Stdout, "%s\n", strings.Join(job.Args, " "))
}

func Exec(job *beam.Job) {
	cmd := exec.Command("/bin/sh", "-c", strings.Join(job.Args, " "))
	cmd.Stdout = job.Stdout
	cmd.Stderr = job.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(job.Stderr, "exec: %s\n", err)
	}
}

func Listen(job *beam.Job) {
	if len(job.Args) < 1 {
		fmt.Fprintf(job.Stderr, "Usage: %s URL\n", job.Name)
		return
	}
	addr, err := url.Parse(job.Args[0])
	if err != nil {
		fmt.Fprintf(job.Stderr, "invalid url: %s\n", job.Args[0])
		return
	}
	l, err := net.Listen(addr.Scheme, addr.Host)
	if err != nil {
		fmt.Fprintf(job.Stderr, "listen: %s\n", err)
		return
	}
	fmt.Fprintf(job.Stdout, "listening on %s\n", job.Args[0])
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Fprintf(job.Stderr, "accept: %s\n", err)
			return
		}
		fmt.Fprintf(job.Stdout, "received new connection from %s\n", conn.RemoteAddr())
		if err := job.Msg.Stream.Send(beam.Message{Data: []byte("conn"), Stream: beam.WrapIO(conn, 0)}); err != nil {
			return
		}
	}
}

func Unknown(job *beam.Job) {
	fmt.Fprintf(job.Stderr, "No such command: %s\n", job.Name)
}
