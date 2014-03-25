package main

import (
	"io"
	"fmt"
	"os"
	"github.com/dotcloud/docker/pkg/dockerscript"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/beam/data"
	"github.com/dotcloud/docker/pkg/term"
	"strings"
	"sync"
	"net"
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
		job, err := beam.SendPipe(client, data.Empty().Set("cmd", cmd.Args...).Bytes())
		if err != nil {
			Fatal(err)
		}
		// Recursively execute child-commands as commands to the new job
		// executeScript blocks until commands are done, so this is depth-first recursion.
		executeScript(job, cmd.Children)
		// TODO: pass a default handler to deal with 'status'
		// --> use beam chaining?
		Debugf("Listening for reply messages\n")
		Serve(job, builtinsHandler)
		Debugf("[%s] done\n", strings.Join(cmd.Args, " "))
	}
}

func parseMsgPayload(payload []byte) ([]string, error) {
	msg, err := data.Decode(string(payload))
	if err != nil {
		return nil, err
	}
	var cmd []string
	if c, exists := msg["cmd"]; exists {
		cmd = c
	}
	if len(cmd) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return cmd, nil
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
	if err := beam.Send(resp, data.Empty().Set("cmd", "log", "stdout").Bytes(), r); err != nil {
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

func CmdTrace(args []string, f *os.File) {
	resp, err := beam.FdConn(int(f.Fd()))
	if err != nil {
		Fatal(err)
		return
	}
	defer resp.Close()
	for {
		Logf("[trace] waiting for a message\n")
		payload, attachment, err := beam.Receive(resp)
		if err != nil {
			Logf("[trace] error waiting for message\n")
			return
		}
		Logf("[trace] received message!\n")
		msg, err := data.Decode(string(payload))
		if err != nil {
			fmt.Printf("===> %s\n", payload)
		} else {
			fmt.Printf("===> %v\n", msg)
		}
		if err := beam.Send(resp, payload, attachment); err != nil {
			return
		}
	}
}


func CmdLog(args []string, f *os.File) {
	defer Debugf("CmdLog done\n")
	var name string
	if len(args) > 0 {
		name = args[1]
	}
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

type Msg struct {
	payload		[]byte
	attachment	*os.File
}

func Serve(endpoint *net.UnixConn, handler func([]string, *os.File)) error {
	Debugf("[Serve %#v]\n", handler)
	defer Debugf("[Serve %#v] done\n", handler)
	var tasks sync.WaitGroup
	defer tasks.Wait()
	in := make(chan *Msg)
	go func() {
		for {
			Debugf("[Serve] waiting for next message on endpoint...\n")
			payload, attachment, err := beam.Receive(endpoint)
			if err != nil {
				break
			}
			in<-&Msg{payload, attachment}
		}
		Debugf("[Serve] endpoint closed. Waiting for tasks to complete\n")
		tasks.Wait()
		Debugf("[Serve] endpoint closed AND tasks complete\n")
		close(in)
	}()
	for msg := range in {
		tasks.Add(1)
		// Handle new message
		go func(payload []byte, attachment *os.File) {
			Debugf("---> Handling '%s' [fd=%d]\n", payload, attachment.Fd())
			defer tasks.Done()
			args, err := parseMsgPayload(payload)
			if err != nil {
				Logf("error parsing beam message: %s\n", err)
				if attachment != nil {
					attachment.Close()
				}
				return
			}
			Debugf("---> calling handler for '%s'\n", args[0])
			handlerAttachment := attachment
			var iotasks sync.WaitGroup
			if attachment != nil {
				if caller, err := beam.FdConn(int(attachment.Fd())); err == nil {
					Debugf("[Serve] message '%s' has a valid beam endpoint as attachment. Setting up indirection\n", payload)
					defer caller.Close()
					jobpub, jobpriv, err := beam.USocketPair()
					if err != nil {
						return
					}
					defer jobpub.Close()
					if f, err := jobpriv.File(); err != nil {
						jobpriv.Close()
						return
					} else {
						handlerAttachment = f
						defer attachment.Close()
					}
					jobpriv.Close()
					// Read messages from the job and re-insert them for handling
					iotasks.Add(1)
					go func(job *net.UnixConn) {
						defer iotasks.Done()
						for {
							payload, attachment, err := beam.Receive(job)
							if err != nil {
								return
							}
							var fd int = -1
							if attachment != nil {
								fd = int(attachment.Fd())
							}
							Debugf("[Serve] received introspection message '%s'[%d]\n", payload, fd)
							// Send messages back in for introspection
							// Note that we don't scope introspection: jobs have full access to the
							// context in which they were called.
							in <-&Msg{payload, attachment}
						}
					}(jobpub)
					// Read messages from the caller to the job
					go func(caller *net.UnixConn, job *net.UnixConn) {
						for {
							payload, f, err := beam.Receive(caller)
							if err != nil {
								return
							}
							if err := beam.Send(job, payload, f); err != nil {
								return
							}
						}
					}(caller, jobpub)
				}
			}
			handler(args, handlerAttachment)
			Debugf("---> handler returned for '%s'\n", args[0])
			if handlerAttachment != nil {
				handlerAttachment.Close()
			}
			Debugf("---> waiting for iotasks to complete for '%s'\n", args[0])
			iotasks.Wait()
			Debugf("---> iotasks complete for '%s'\n", args[0])
		}(msg.payload, msg.attachment)
	}
	Debugf("[Serve] main serve loop completed\n")
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
	} else if args[0] == "trace" {
		CmdTrace(args, attachment)
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
