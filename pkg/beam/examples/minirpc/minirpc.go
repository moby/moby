package main

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	hub := Hub()
	commands := Commands()
	cli := Cli(os.Stdin, os.Stdout, os.Stderr)
	hub.Send(beam.Message{Data: []byte("register exec echo listen"), Stream: commands})
	beam.Copy(hub, cli)
}

func Cli(stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) beam.Stream {
	inside, outside := beam.Pipe()
	input := bufio.NewScanner(stdin)
	var tasks sync.WaitGroup
	go func() {
		defer tasks.Wait()
		defer inside.Close()
		for id := 0; ; id++ {
			if !input.Scan() {
				return
			}
			local, remote := beam.Pipe()
			msg := beam.Message{
				Data:   input.Bytes(),
				Stream: remote,
			}
			if len(msg.Data) == 0 && input.Err() == nil {
				continue
			}
			tasks.Add(1)
			go func(id int) {
				fmt.Printf("New task with id=%d\n", id)
				defer tasks.Done()
				defer local.Close()
				for i := 0; true; i++ {
					m, err := local.Receive()
					if err != nil {
						return
					}
					fmt.Fprintf(stdout, "[%d] %s\n", id, m.Data)
					if m.Stream == nil {
						continue
					}
					name := string(m.Data)
					prefix := fmt.Sprintf("[%d] [%s] ", id, name)
					var output io.Writer
					if name == "stderr" {
						output = prefixer(prefix, stderr)
					} else {
						output = prefixer(prefix, stdout)
					}
					tasks.Add(1)
					go func() {
						io.Copy(output, beam.NewReader(m.Stream))
						fmt.Fprintf(output, "<EOF>\n")
						tasks.Done()
					}()
				}
			}(id)
			if err := inside.Send(msg); err != nil {
				return
			}
		}
	}()
	return outside
}

func prefixer(prefix string, output io.Writer) io.Writer {
	r, w := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := prefix + scanner.Text()
			if len(line) == 0 || line[len(line)-1] != '\n' {
				line = line + "\n"
			}
			fmt.Fprintf(output, "%s", line)
		}
	}()
	return w
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
				go JobHandler(Echo).Send(msg)
			} else if name == "exec" {
				go JobHandler(Exec).Send(msg)
			} else if name == "listen" {
				go JobHandler(Listen).Send(msg)
			} else {
				go JobHandler(Unknown).Send(msg)
			}
		}
	}()
	return outside
}

func JobHandler(f func(*Job)) beam.Stream {
	inside, outside := beam.Pipe()
	go func() {
		defer inside.Close()
		for {
			msg, err := inside.Receive()
			if err != nil {
				return
			}
			if msg.Stream == nil {
				msg.Stream = beam.DevNull
			}
			parts := strings.Split(string(msg.Data), " ")
			// Setup default stdout
			// FIXME: The job handler can change it before calling job.Send()
			// For example if it wants to send a file (eg. 'exec')
			func() error {
				stdout, stdoutStream := PipeWriter()
				if err := msg.Stream.Send(beam.Message{Data: []byte("stdout"), Stream: stdoutStream}); err != nil {
					return err
				}
				stderr, stderrStream := PipeWriter()
				if err := msg.Stream.Send(beam.Message{Data: []byte("stderr"), Stream: stderrStream}); err != nil {
					return err
				}
				job := &Job{
					Msg:    msg,
					Name:   parts[0],
					Stdout: stdout,
					Stderr: stderr,
				}
				if len(parts) > 1 {
					job.Args = parts[1:]
				}
				f(job)
				return nil
			}()
		}
	}()
	return outside
}

func PipeWriter() (io.WriteCloser, beam.Stream) {
	r, w := io.Pipe()
	inside, outside := beam.Pipe()
	go func() {
		defer inside.Close()
		defer r.Close()
		for {
			data := make([]byte, 4098)
			n, err := r.Read(data)
			if n > 0 {
				if inside.Send(beam.Message{Data: data[:n]}); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return w, outside
}

type Job struct {
	Msg    beam.Message
	Name   string
	Args   []string
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func Echo(job *Job) {
	fmt.Fprintf(job.Stdout, "%s\n", strings.Join(job.Args, " "))
}

func Exec(job *Job) {
	cmd := exec.Command("/bin/sh", "-c", strings.Join(job.Args, " "))
	cmd.Stdout = job.Stdout
	cmd.Stderr = job.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(job.Stderr, "exec: %s\n", err)
	}
}

func Listen(job *Job) {
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

func Unknown(job *Job) {
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
				JobHandler(func(job *Job) {
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
