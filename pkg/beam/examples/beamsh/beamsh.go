package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/beam/data"
	"github.com/dotcloud/docker/pkg/dockerscript"
	"github.com/dotcloud/docker/pkg/term"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

func main() {
	devnull, err := Devnull()
	if err != nil {
		Fatal(err)
	}
	defer devnull.Close()
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
				executeScript(devnull, cmd)
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
		executeScript(devnull, script)
	}
}

func beamCopy(dst *net.UnixConn, src *net.UnixConn) error {
	for {
		payload, attachment, err := beam.Receive(src)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err := beam.Send(dst, payload, attachment); err != nil {
			if attachment != nil {
				attachment.Close()
			}
			return err
		}
	}
	panic("impossibru!")
	return nil
}

type Handler func([]string, *net.UnixConn, *net.UnixConn)

func Devnull() (*net.UnixConn, error) {
	priv, pub, err := beam.USocketPair()
	if err != nil {
		return nil, err
	}
	go func() {
		defer priv.Close()
		for {
			payload, attachment, err := beam.Receive(priv)
			if err != nil {
				return
			}
			fmt.Fprintf(os.Stderr, "[devnull] discarding '%s'\n", data.Message(string(payload)).Pretty())
			if attachment != nil {
				attachment.Close()
			}
		}
	}()
	return pub, nil
}

func scriptString(script []*dockerscript.Command) string {
	lines := make([]string, 0, len(script))
	for _, cmd := range script {
		line := strings.Join(cmd.Args, " ")
		if len(cmd.Children) > 0 {
			line += fmt.Sprintf(" { %s }", scriptString(cmd.Children))
		} else {
			line += " {}"
		}
		lines = append(lines, line)
	}
	return fmt.Sprintf("'%s'", strings.Join(lines, "; "))
}

func executeScript(client *net.UnixConn, script []*dockerscript.Command) error {
	Debugf("executeScript(%s)\n", scriptString(script))
	defer Debugf("executeScript(%s) DONE\n", scriptString(script))
	for _, cmd := range script {
		if err := executeCommand(client, cmd); err != nil {
			return err
		}
	}
	return nil
}

//	1) Find a handler for the command (if no handler, fail)
//	2) Attach new in & out pair to the handler
//	3) [in the background] Copy handler output to our own output
//	4) [in the background] Run the handler
//	5) Recursively executeScript() all children commands and wait for them to complete
//	6) Wait for handler to return and (shortly afterwards) output copy to complete
//	7) 
func executeCommand(client *net.UnixConn, cmd *dockerscript.Command) error {
	Debugf("executeCommand(%s)\n", strings.Join(cmd.Args, " "))
	defer Debugf("executeCommand(%s) DONE\n", strings.Join(cmd.Args, " "))
	handler := GetHandler(cmd.Args[0])
	if handler == nil {
		return fmt.Errorf("no such command: %s", cmd.Args[0])
	}
	inPub, inPriv, err := beam.USocketPair()
	if err != nil {
		return err
	}
	// Don't close inPub here. We close it to signify the end of input once
	// all children are completed (guaranteeing that no more input will be sent
	// by children).
	// Otherwise we get a deadlock.
	defer inPriv.Close()
	outPub, outPriv, err := beam.USocketPair()
	if err != nil {
		return err
	}
	defer outPub.Close()
	// don't close outPriv here. It must be closed after the handler is called,
	// but before the copy tasks associated with it completes.
	// Otherwise we get a deadlock.
	var tasks sync.WaitGroup
	tasks.Add(2)
	go func() {
		handler(cmd.Args, inPriv, outPriv)
		// FIXME: do we need to outPriv.sync before closing it?
		Debugf("[%s] handler returned, closing output\n", strings.Join(cmd.Args, " "))
		outPriv.Close()
		tasks.Done()
	}()
	go func() {
		Debugf("[%s] copy start...\n", strings.Join(cmd.Args, " "))
		beamCopy(client, outPub)
		Debugf("[%s] copy done\n", strings.Join(cmd.Args, " "))
		tasks.Done()
	}()
	// depth-first execution of children commands
	// executeScript() blocks until all commands are completed
	executeScript(inPub, cmd.Children)
	inPub.Close()
	Debugf("[%s] waiting for handler and output copy to complete...\n", strings.Join(cmd.Args, " "))
	tasks.Wait()
	Debugf("[%s] handler and output copy complete!\n", strings.Join(cmd.Args, " "))
	return nil
}

func randomId() string {
	id := make([]byte, 4)
	io.ReadFull(rand.Reader, id)
	return hex.EncodeToString(id)
}

func GetHandler(name string) Handler {
	if name == "exec" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			cmd := exec.Command(args[1], args[2:]...)
			outR, outW, err := os.Pipe()
			if err != nil {
				return
			}
			cmd.Stdout = outW
			errR, errW, err := os.Pipe()
			if err != nil {
				return
			}
			cmd.Stderr = errW
			beam.Send(out, data.Empty().Set("cmd", "log", "stdout").Bytes(), outR)
			beam.Send(out, data.Empty().Set("cmd", "log", "stderr").Bytes(), errR)
			execErr := cmd.Run()
			var status string
			if execErr != nil {
				status = execErr.Error()
			} else {
				status = "ok"
			}
			beam.Send(out, data.Empty().Set("status", status).Set("cmd", args...).Bytes(), nil)
		}
	} else if name == "trace" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			for {
				p, a, err := beam.Receive(in)
				if err != nil {
					return
				}
				var msg string
				if pretty := data.Message(string(p)).Pretty(); pretty != "" {
					msg = pretty
				} else {
					msg = string(p)
				}
				if a != nil {
					msg = fmt.Sprintf("%s [%d]", msg, a.Fd())
				}
				fmt.Printf("===> %s\n", msg)
				beam.Send(out, p, a)
			}
		}
	} else if name == "emit" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			beam.Send(out, data.Empty().Set("foo", args[1:]...).Bytes(), nil)
		}
	} else if name == "print" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			for {
				_, a, err := beam.Receive(in)
				if err != nil {
					return
				}
				if a != nil {
					io.Copy(os.Stdout, a)
				}
			}
		}
	} else if name == "multiprint" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			var tasks sync.WaitGroup
			for {
				payload, a, err := beam.Receive(in)
				if err != nil {
					return
				}
				if a != nil {
					tasks.Add(1)
					go func(payload []byte, attachment *os.File) {
						defer tasks.Done()
						msg := data.Message(string(payload))
						input := bufio.NewScanner(attachment)
						for input.Scan() {
							fmt.Printf("[%s] %s\n", msg.Pretty(), input.Text())
						}
					}(payload, a)
				}
			}
			tasks.Wait()
		}
	} else if name == "listen" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			if len(args) != 2 {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil)
				return
			}
			u, err := url.Parse(args[1])
			if err != nil {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			l, err := net.Listen(u.Scheme, u.Host)
			if err != nil {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			for {
				conn, err := l.Accept()
				if err != nil {
					beam.Send(out, data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
					return
				}
				f, err := connToFile(conn)
				if err != nil {
					conn.Close()
					continue
				}
				beam.Send(out, data.Empty().Set("type", "socket").Set("remoteaddr", conn.RemoteAddr().String()).Bytes(), f)
			}
		}
	} else if name == "connect" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			if len(args) != 2 {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil)
				return
			}
			u, err := url.Parse(args[1])
			if err != nil {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			var tasks sync.WaitGroup
			for {
				_, attachment, err := beam.Receive(in)
				if err != nil {
					break
				}
				if attachment == nil {
					continue
				}
				Logf("connecting to %s/%s\n", u.Scheme, u.Host)
				conn, err := net.Dial(u.Scheme, u.Host)
				if err != nil {
					beam.Send(out, data.Empty().Set("cmd", "msg", "connect error: " + err.Error()).Bytes(), nil)
					return
				}
				beam.Send(out, data.Empty().Set("cmd", "msg", "connection established").Bytes(), nil)
				tasks.Add(1)
				go func(attachment *os.File, conn net.Conn) {
					defer tasks.Done()
					// even when successful, conn.File() returns a duplicate,
					// so we must close the original
					var iotasks sync.WaitGroup
					iotasks.Add(2)
					go func(attachment *os.File, conn net.Conn) {
						defer iotasks.Done()
						io.Copy(attachment, conn)
					}(attachment, conn)
					go func(attachment *os.File, conn net.Conn) {
						defer iotasks.Done()
						io.Copy(conn, attachment)
					}(attachment, conn)
					iotasks.Wait()
					conn.Close()
					attachment.Close()
				}(attachment, conn)
			}
			tasks.Wait()
		}
	} else if name == "openfile" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			for _, name := range args {
				f, err := os.Open(name)
				if err != nil {
					continue
				}
				if err := beam.Send(out, data.Empty().Set("path", name).Set("type", "file").Bytes(), f); err != nil {
					f.Close()
				}
			}
		}
	}
	return nil
}

func connToFile(conn net.Conn) (f *os.File, err error) {
	if connWithFile, ok := conn.(interface { File() (*os.File, error) }); !ok {
		return nil, fmt.Errorf("no file descriptor available")
	} else {
		f, err = connWithFile.File()
		if err != nil {
			return nil, err
		}
	}
	return f, err
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

type Msg struct {
	payload		[]byte
	attachment	*os.File
}

func Logf(msg string, args ...interface{}) (int, error) {
	if len(msg) == 0 || msg[len(msg) - 1] != '\n' {
		msg = msg + "\n"
	}
	msg = fmt.Sprintf("[%v] [%v] %s", os.Getpid(), path.Base(os.Args[0]), msg)
	return fmt.Printf(msg, args...)
}

func Debugf(msg string, args ...interface{}) {
	if os.Getenv("BEAMDEBUG") != "" {
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
