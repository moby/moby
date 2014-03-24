package main

import (
	"io"
	"fmt"
	"os"
	"github.com/dotcloud/docker/pkg/dockerscript"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/term"
	"strings"
	"sync"
	"net"
	"bytes"
	"path"
	"bufio"
	"strconv"
)

func main() {
	Debugf("Initializing engine\n")
	client, engine, err := beam.USocketPair()
	if err != nil {
		Fatal(err)
	}
	defer client.Close()
	go func() {
		Serve(engine, builtinsHandler)
		Debugf("Shutting down engine\n")
		engine.Close()
	}()
	if term.IsTerminal(0) {
		input := bufio.NewScanner(os.Stdin)
		for {
			os.Stdout.Write([]byte("beamsh> "))
			if !input.Scan() {
				break
			}
			line := input.Text()
			if len(line) != 0 {
				cmd, err := dockerscript.Parse(strings.NewReader(line))
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					continue
				}
				executeScript(client, cmd)
			}
			if err := input.Err(); err == io.EOF {
				break
			} else if err != nil {
				Fatal(err)
			}
		}
	} else {
		script, err := dockerscript.Parse(os.Stdin)
		if err != nil {
			Fatal("parse error: %v\n", err)
		}
		executeScript(client, script)
	}
}

func executeScript(client *net.UnixConn, script []*dockerscript.Command) {
	Debugf("%d commands:\n", len(script))
	for _, cmd := range script {
		job, err := beam.SendPipe(client, []byte(strings.Join(cmd.Args, " ")))
		if err != nil {
			Fatal(err)
		}
		// TODO: pass a default handler to deal with 'status'
		// --> use beam chaining?
		Debugf("Listening for reply messages\n")
		Serve(job, builtinsHandler)
		Debugf("[%s] done\n", strings.Join(cmd.Args, " "))
	}
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
		Fatal(err)
		return
	}
	defer resp.Close()
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	Debugf("[CmdEcho] stdout pipe() r=%d w=%d\n", r.Fd(), w.Fd())
	if err := beam.Send(resp, []byte("log stdout"), r); err != nil {
		return
	}
	fmt.Fprintln(w, strings.Join(args[1:], " "))
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

// 'status' is a notification of a job's status.
// 
func parseEnv(args []string) ([]string, map[string]string) {
	var argsOut []string
	env := make(map[string]string)
	for _, word := range args[1:] {
		if strings.Contains(word, "=") {
			kv := strings.SplitN(word, "=", 2)
			key := kv[0]
			var val string
			if len(kv) == 2 {
				val = kv[1]
			}
			env[key] = val
		} else {
			argsOut = append(argsOut, word)
		}
	}
	return argsOut, env
}

func CmdLog(args []string, f *os.File) {
	defer Debugf("CmdLog done\n")
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

func Serve(endpoint *net.UnixConn, handler func([]string, *os.File)) error {
	Debugf("[Serve %#v]\n", handler)
	defer Debugf("[Serve %#v] done\n", handler)
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for {
		payload, attachment, err := beam.Receive(endpoint)
		if err != nil {
			return err
		}
		tasks.Add(1)
		// Handle new message
		go func(payload []byte, attachment *os.File) {
			Debugf("---> Handling '%s' [fd=%d]\n", payload, attachment.Fd())
			defer tasks.Done()
			if attachment != nil {
				defer func() {
					Debugf("---> Closing attachment [fd=%d] for msg '%s'\n", attachment.Fd(), payload)
					attachment.Close()
				}()
			} else {
				defer Debugf("---> No attachment to close for msg '%s'\n", payload)
			}
			args, err := parseMsgPayload(payload)
			if err != nil {
				Logf("error parsing beam message: %s\n", err)
				if attachment != nil {
					attachment.Close()
				}
				return
			}
			Debugf("---> calling handler for '%s'\n", args[0])
			handler(args, attachment)
			Debugf("---> handler returned for '%s'\n", args[0])
		}(payload, attachment)
	}
	return nil
}

func builtinsHandler(args []string, attachment *os.File) {
	if args[0] == "exit" {
		CmdExit(args, attachment)
	} else if args[0] == "cat" {
		CmdCat(args, attachment)
	} else if args[0] == "echo" {
		CmdEcho(args, attachment)
	} else if args[0] == "log" {
		CmdLog(args, attachment)
	}
}


func Logf(msg string, args ...interface{}) (int, error) {
	if len(msg) == 0 || msg[len(msg) - 1] != '\n' {
		msg = msg + "\n"
	}
	msg = fmt.Sprintf("[%v] [%v] %s", os.Getpid(), path.Base(os.Args[0]), msg)
	return fmt.Printf(msg, args...)
}

func Debugf(msg string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		Logf(msg, args...)
	}
}

func Fatalf(msg string, args ...interface{})  {
	Logf(msg, args)
	os.Exit(1)
}

func Fatal(args ...interface{}) {
	Fatalf("%v", args[0])
}
