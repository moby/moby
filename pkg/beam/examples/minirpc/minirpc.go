package main

import (
	"bufio"
	"fmt"
	"io"
	"github.com/dotcloud/docker/pkg/beam"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"net"
	"net/url"
)

func main() {
	runCommand := beam.Func(RunCommand)
	cli := NewCLI(os.Stdin, os.Stdout, os.Stderr)
	if err := beam.Splice(cli, runCommand); err != nil {
		die("splice: %s", err)
	}
}

type CLI struct {
	input *bufio.Scanner
	stdin io.ReadCloser
	stdout io.Writer
	stderr io.Writer
	tasks sync.WaitGroup
}

func (cli *CLI) Receive() (msg beam.Message, err error) {
	for {
		if !cli.input.Scan() {
			fmt.Printf("done!\n")
			err = io.EOF
			return
		}
		msg.Data = cli.input.Bytes()
		err = cli.input.Err()
		if len(msg.Data) == 0 && err == nil {
			continue
		}
		local, remote := beam.Pipe()
		cli.tasks.Add(1)
		go func() {
			defer cli.tasks.Done()
			defer local.Close()
			for {
				m, err := local.Receive()
				if err != nil {
					return
				}
				if m.Stream == nil {
					continue
				}
				name := string(m.Data)
				var output io.Writer
				if name == "stdout" {
					output = cli.stdout
				} else if name == "stderr" {
					output = cli.stderr
				} else {
					prefix := fmt.Sprintf("[%s]--->[%s] ", msg.Data, name)
					output = prefixer(prefix, cli.stderr)
				}
				cli.tasks.Add(1)
				go func() {
					io.Copy(output, beam.NewReader(m.Stream))
					fmt.Fprintf(cli.stderr, "[EOF] %s\n", m.Data)
					cli.tasks.Done()
				}()
			}
		}()
		msg.Stream = remote
		break
	}
	return
}

func prefixer(prefix string, output io.Writer) io.Writer {
	r, w := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			fmt.Fprintf(output, "%s%s\n", prefix, scanner.Bytes())
		}
	}()
	return w
}

func (cli *CLI) Send(msg beam.Message) (err error) {
	if msg.Stream == nil {
		return nil
	}
	fmt.Printf("---> CLI received new stream (%s)\n", msg.Data)
	go beam.Splice(msg.Stream, beam.DevNull)
	return nil
}

func (cli *CLI) Close() error {
	cli.tasks.Wait()
	return nil
}

func (cli *CLI) File() (*os.File, error) {
	return nil, fmt.Errorf("no file descriptor available for this stream")
}

func NewCLI(stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) *CLI {
	return &CLI{
		stdin: stdin,
		input: bufio.NewScanner(stdin),
		stdout: stdout,
		stderr: stderr,
	}
}

func RunCommand(msg beam.Message) (err error) {
	parts := strings.Split(string(msg.Data), " ")
	// Setup stdout
	stdoutRemote, stdout := io.Pipe()
	defer stdout.Close()
	if err := msg.Stream.Send(beam.Message{Data: []byte("stdout"), Stream: beam.WrapIO(stdoutRemote, 0)}); err != nil {
		return err
	}
	// Setup stderr
	stderrRemote, stderr := io.Pipe()
	defer stderr.Close()
	if err := msg.Stream.Send(beam.Message{Data: []byte("stderr"), Stream: beam.WrapIO(stderrRemote, 0)}); err != nil {
		return err
	}
	if parts[0] == "echo" {
		fmt.Fprintf(stdout, "%s\n", strings.Join(parts[1:], " "))
	} else if parts[0] == "die" {
		os.Exit(1)
	} else if parts[0] == "sleep" {
		time.Sleep(1 * time.Second)
	} else if parts[0] == "exec" {
		cmd := exec.Command("/bin/sh", "-c", strings.Join(parts[1:], " "))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(stderr, "exec: %s\n", err)
		}
	} else if parts[0] == "listen" {
		if len(parts) < 2 {
			fmt.Fprintf(stderr, "Usage: %s URL\n", parts[0])
			return nil
		}
		addr, err := url.Parse(parts[1])
		if err != nil {
			fmt.Fprintf(stderr, "invalid url: %s\n", parts[1])
			return nil
		}
		l, err := net.Listen(addr.Scheme, addr.Host)
		if err != nil {
			fmt.Fprintf(stderr, "listen: %s\n", err)
			return nil
		}
		fmt.Fprintf(stdout, "listening on %s\n", parts[1])
		for {
			conn, err := l.Accept()
			if err != nil {
				fmt.Fprintf(stderr, "accept: %s\n", err)
				return nil
			}
			fmt.Fprintf(stdout, "received new connection from %s\n", conn.RemoteAddr())
			if err := msg.Stream.Send(beam.Message{Data: []byte("conn"), Stream: beam.WrapIO(conn, 0)}); err != nil {
				return fmt.Errorf("send: %s\n", err)
			}
		}

	} else {
		fmt.Fprintf(stderr, "%s: command not found\n", parts[0])
	}
	return nil
}

func die(f string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, f, args...)
	os.Exit(1)
}
