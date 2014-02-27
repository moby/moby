package main

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func main() {
	hub := Hub()
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

func Hub() beam.Stream {
	inside, outside := beam.Pipe()
	go func() {
		defer fmt.Printf("hub stopped\n")
		handlers := make(map[string]beam.Stream)
		for {
			msg, err := inside.Receive()
			if err != nil {
				return
			}
			if msg.Stream == nil {
				continue
			}
			// FIXME: use proper word parsing
			words := strings.Split(string(msg.Data), " ")
			if words[0] == "register" {
				if len(words) < 2 {
					msg.Stream.Send(beam.Message{Data: []byte("Usage: register COMMAND\n")})
					msg.Stream.Close()
				}
				for _, cmd := range words[1:] {
					fmt.Printf("Registering handler for %s\n", cmd)
					handlers[cmd] = msg.Stream
				}
				msg.Stream.Send(beam.Message{Data: []byte("test on registered handler\n")})
			} else if words[0] == "commands" {
				beam.JobHandler(func(job *beam.Job) {
					fmt.Fprintf(job.Stdout, "%d commands:\n", len(handlers))
					for cmd := range handlers {
						fmt.Fprintf(job.Stdout, "%s\n", cmd)
					}
				}).Send(msg)
			} else if handler, exists := handlers[words[0]]; exists {
				err := handler.Send(msg)
				if err != nil {
					log.Printf("Error sending to %s handler: %s. De-registering handler.\n", words[0], err)
					delete(handlers, words[0])
				}
			} else {
				msg.Stream.Send(beam.Message{Data: []byte("No such command: " + words[0] + "\n")})
			}
		}
	}()
	return outside
}
