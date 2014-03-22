package main

import (
	"io"
	"fmt"
	"os"
	"github.com/dotcloud/docker/pkg/dockerscript"
	"github.com/dotcloud/docker/pkg/beam"
	"strings"
	"sync"
	"net"
	"bytes"
	"path"
	"bufio"
	"strconv"
)

func main() {
	script, err := dockerscript.Parse(os.Stdin)
	if err != nil {
		Fatal("parse error: %v\n", err)
	}
	Logf("%d commands:\n", len(script))
	client, engine, err := beam.USocketPair()
	if err != nil {
		Fatal(err)
	}
	go func() {
		Serve(engine)
		engine.Close()
	}()
	for _, cmd := range script {
		job, err := beam.SendPipe(client, []byte(strings.Join(cmd.Args, " ")))
		if err != nil {
			Fatal(err)
		}
		Serve(job)
		Logf("[%s] done\n", strings.Join(cmd.Args, " "))
	}
	client.Close()
}

func parseMsgPayload(payload []byte) ([]string, error) {
	// FIXME: send structured message instead of a text script
	cmds, err := dockerscript.Parse(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if len(cmds) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	// We don't care about multiple commands or children
	return cmds[0].Args, nil
}

func CmdCat(args []string, f *os.File) {
	for _, name := range args[1:] {
		f, err := os.Open(name)
		if err != nil {
			continue
		}
		io.Copy(os.Stdout, f)
		f.Close()
	}
}

func CmdEcho(args []string, f *os.File) {
	resp, err := beam.FdConn(int(f.Fd()))
	if err != nil {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	if err := beam.Send(resp, []byte("log stdout"), r); err != nil {
		return
	}
	fmt.Fprintln(w, strings.Join(args[1:], " "))
	fmt.Printf("waiting 5 seconds...\n")
	w.Close()
}

func CmdExit(args []string, f *os.File) {
	var status int
	if len(args) > 1 {
		val, err := strconv.ParseInt(args[1], 10, 32)
		if err == nil {
			status = int(val)
		}
	}
	os.Exit(status)
}

func CmdLog(args []string, f *os.File) {
	name := args[1]
	input := bufio.NewScanner(f)
	for input.Scan() {
		line := input.Text()
		if len(line) > 0 {
			fmt.Printf("[%s] %s\n", name, line)
		}
		if err := input.Err(); err != nil {
			fmt.Printf("[%s:%s]\n", name, err)
			break
		}
	}
}

func Serve(endpoint *net.UnixConn) error {
	Logf("[Serve]\n")
	defer Logf("[Serve] done\n")
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for {
		Logf("[Serve] waiting for next message...\n")
		payload, attachment, err := beam.Receive(endpoint)
		if err != nil {
			return err
		}
		tasks.Add(1)
		go func(payload []byte, attachment *os.File) {
			defer tasks.Done()
			if attachment != nil {
				defer fmt.Printf("closing request attachment %d\n", attachment.Fd())
				attachment.Close()
			}
			args, err := parseMsgPayload(payload)
			if err != nil {
				Logf("error parsing beam message: %s\n", err)
				if attachment != nil {
					attachment.Close()
				}
				return
			}
			Logf("beam message: %v\n", args)
			if args[0] == "exit" {
				CmdExit(args, attachment)
			} else if args[0] == "cat" {
				CmdCat(args, attachment)
			} else if args[0] == "echo" {
				CmdEcho(args, attachment)
			} else if args[0] == "log" {
				CmdLog(args, attachment)
			}
		}(payload, attachment)
	}
	return nil
}


func Logf(msg string, args ...interface{}) (int, error) {
	if len(msg) == 0 || msg[len(msg) - 1] != '\n' {
		msg = msg + "\n"
	}
	msg = fmt.Sprintf("[%v] [%v] %s", os.Getpid(), path.Base(os.Args[0]), msg)
	return fmt.Printf(msg, args...)
}

func Fatalf(msg string, args ...interface{})  {
	Logf(msg, args)
	os.Exit(1)
}

func Fatal(args ...interface{}) {
	Fatalf("%v", args[0])
}
