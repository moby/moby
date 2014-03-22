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
		// Synchronize on the job handler
		for {
			_, _, err := beam.Receive(job)
			if err == io.EOF {
				break
			} else if err != nil {
				Fatalf("error reading from job handler: %#v\n", err)
			}
		}
		Logf("[%s] done\n", strings.Join(cmd.Args, " "))
	}
	client.Close()
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
	fmt.Println(strings.Join(args[1:], " "))
}

func CmdDie(args []string, f *os.File) {
	Fatal(strings.Join(args[1:], " "))
}

func Serve(endpoint *net.UnixConn) error {
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for {
		payload, attachment, err := beam.Receive(endpoint)
		if err != nil {
			return err
		}
		tasks.Add(1)
		go func(payload []byte, attachment *os.File) {
			defer tasks.Done()
			defer func() {
				if attachment != nil {
					attachment.Close()
				}
			}()
			// FIXME: send structured message instead of a text script
			cmds, err := dockerscript.Parse(bytes.NewReader(payload))
			if err != nil {
				Logf("error parsing beam message: %s\n", err)
				if attachment != nil {
					attachment.Close()
				}
				return
			}
			if len(cmds) == 0 {
				Logf("ignoring empty command\n", err)
			}
			// We don't care about multiple commands or children
			args := cmds[0].Args
			Logf("beam message: %v\n", args)
			if args[0] == "die" {
				CmdDie(args, attachment)
			} else if args[0] == "cat" {
				CmdCat(args, attachment)
			} else if args[0] == "echo" {
				CmdEcho(args, attachment)
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
