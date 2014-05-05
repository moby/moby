package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/beam/data"
	"github.com/dotcloud/docker/pkg/dockerscript"
	"github.com/dotcloud/docker/pkg/term"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
)

var rootPlugins = []string{
	"stdio",
}

var (
	flX        bool
	flPing     bool
	introspect beam.ReceiveSender = beam.Devnull()
)

func main() {
	fd3 := os.NewFile(3, "beam-introspect")
	if introsp, err := beam.FileConn(fd3); err == nil {
		introspect = introsp
		Logf("introspection enabled\n")
	} else {
		Logf("introspection disabled\n")
	}
	fd3.Close()
	flag.BoolVar(&flX, "x", false, "print commands as they are being executed")
	flag.Parse()
	if flag.NArg() == 0 {
		if term.IsTerminal(0) {
			// No arguments, stdin is terminal --> interactive mode
			input := bufio.NewScanner(os.Stdin)
			for {
				fmt.Printf("[%d] beamsh> ", os.Getpid())
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
					if err := executeRootScript(cmd); err != nil {
						Fatal(err)
					}
				}
				if err := input.Err(); err == io.EOF {
					break
				} else if err != nil {
					Fatal(err)
				}
			}
		} else {
			// No arguments, stdin not terminal --> batch mode
			script, err := dockerscript.Parse(os.Stdin)
			if err != nil {
				Fatal("parse error: %v\n", err)
			}
			if err := executeRootScript(script); err != nil {
				Fatal(err)
			}
		}
	} else {
		// 1+ arguments: parse them as script files
		for _, scriptpath := range flag.Args() {
			f, err := os.Open(scriptpath)
			if err != nil {
				Fatal(err)
			}
			script, err := dockerscript.Parse(f)
			if err != nil {
				Fatal("parse error: %v\n", err)
			}
			if err := executeRootScript(script); err != nil {
				Fatal(err)
			}
		}
	}
}

func executeRootScript(script []*dockerscript.Command) error {
	if len(rootPlugins) > 0 {
		// If there are root plugins, wrap the script inside them
		var (
			rootCmd *dockerscript.Command
			lastCmd *dockerscript.Command
		)
		for _, plugin := range rootPlugins {
			pluginCmd := &dockerscript.Command{
				Args: []string{plugin},
			}
			if rootCmd == nil {
				rootCmd = pluginCmd
			} else {
				lastCmd.Children = []*dockerscript.Command{pluginCmd}
			}
			lastCmd = pluginCmd
		}
		lastCmd.Children = script
		script = []*dockerscript.Command{rootCmd}
	}
	handlers, err := Handlers(introspect)
	if err != nil {
		return err
	}
	defer handlers.Close()
	var tasks sync.WaitGroup
	defer func() {
		Debugf("Waiting for introspection...\n")
		tasks.Wait()
		Debugf("DONE Waiting for introspection\n")
	}()
	if introspect != nil {
		tasks.Add(1)
		go func() {
			Debugf("starting introspection\n")
			defer Debugf("done with introspection\n")
			defer tasks.Done()
			introspect.Send(data.Empty().Set("cmd", "log", "stdout").Set("message", "introspection worked!").Bytes(), nil)
			Debugf("XXX starting reading introspection messages\n")
			r := beam.NewRouter(handlers)
			r.NewRoute().All().Handler(func(p []byte, a *os.File) error {
				Logf("[INTROSPECTION] %s\n", beam.MsgDesc(p, a))
				return handlers.Send(p, a)
			})
			n, err := beam.Copy(r, introspect)
			Debugf("XXX done reading %d introspection messages: %v\n", n, err)
		}()
	}
	if err := executeScript(handlers, script); err != nil {
		return err
	}
	return nil
}

func executeScript(out beam.Sender, script []*dockerscript.Command) error {
	Debugf("executeScript(%s)\n", scriptString(script))
	defer Debugf("executeScript(%s) DONE\n", scriptString(script))
	var background sync.WaitGroup
	defer background.Wait()
	for _, cmd := range script {
		if cmd.Background {
			background.Add(1)
			go func(out beam.Sender, cmd *dockerscript.Command) {
				executeCommand(out, cmd)
				background.Done()
			}(out, cmd)
		} else {
			if err := executeCommand(out, cmd); err != nil {
				return err
			}
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
//	7) Profit
func executeCommand(out beam.Sender, cmd *dockerscript.Command) error {
	if flX {
		fmt.Printf("+ %v\n", strings.Replace(strings.TrimRight(cmd.String(), "\n"), "\n", "\n+ ", -1))
	}
	Debugf("executeCommand(%s)\n", strings.Join(cmd.Args, " "))
	defer Debugf("executeCommand(%s) DONE\n", strings.Join(cmd.Args, " "))
	if len(cmd.Args) == 0 {
		return fmt.Errorf("empty command")
	}
	Debugf("[executeCommand] sending job '%s'\n", strings.Join(cmd.Args, " "))
	job, err := beam.SendConn(out, data.Empty().Set("cmd", cmd.Args...).Set("type", "job").Bytes())
	if err != nil {
		return fmt.Errorf("%v\n", err)
	}
	var tasks sync.WaitGroup
	tasks.Add(1)
	Debugf("[executeCommand] spawning background copy of the output of '%s'\n", strings.Join(cmd.Args, " "))
	go func() {
		if out != nil {
			Debugf("[executeCommand] background copy of the output of '%s'\n", strings.Join(cmd.Args, " "))
			n, err := beam.Copy(out, job)
			if err != nil {
				Fatalf("[executeCommand] [%s] error during background copy: %v\n", strings.Join(cmd.Args, " "), err)
			}
			Debugf("[executeCommand] background copy done of the output of '%s': copied %d messages\n", strings.Join(cmd.Args, " "), n)
		}
		tasks.Done()
	}()
	// depth-first execution of children commands
	// executeScript() blocks until all commands are completed
	Debugf("[executeCommand] recursively running children of '%s'\n", strings.Join(cmd.Args, " "))
	executeScript(job, cmd.Children)
	Debugf("[executeCommand] DONE recursively running children of '%s'\n", strings.Join(cmd.Args, " "))
	job.CloseWrite()
	Debugf("[executeCommand] closing the input of '%s' (all children are completed)\n", strings.Join(cmd.Args, " "))
	Debugf("[executeCommand] waiting for background copy of '%s' to complete...\n", strings.Join(cmd.Args, " "))
	tasks.Wait()
	Debugf("[executeCommand] background copy of '%s' complete! This means the job completed.\n", strings.Join(cmd.Args, " "))
	return nil
}

type Handler func([]string, io.Writer, io.Writer, beam.Receiver, beam.Sender)

func Handlers(sink beam.Sender) (*beam.UnixConn, error) {
	var tasks sync.WaitGroup
	pub, priv, err := beam.USocketPair()
	if err != nil {
		return nil, err
	}
	go func() {
		defer func() {
			Debugf("[handlers] closewrite() on endpoint\n")
			// FIXME: this is not yet necessary but will be once
			// there is synchronization over standard beam messages
			priv.CloseWrite()
			Debugf("[handlers] done closewrite() on endpoint\n")
		}()
		r := beam.NewRouter(sink)
		r.NewRoute().HasAttachment().KeyIncludes("type", "job").Handler(func(payload []byte, attachment *os.File) error {
			conn, err := beam.FileConn(attachment)
			if err != nil {
				attachment.Close()
				return err
			}
			// attachment.Close()
			tasks.Add(1)
			go func() {
				defer tasks.Done()
				defer func() {
					Debugf("[handlers] '%s' closewrite\n", payload)
					conn.CloseWrite()
					Debugf("[handlers] '%s' done closewrite\n", payload)
				}()
				cmd := data.Message(payload).Get("cmd")
				Debugf("[handlers] received %s\n", strings.Join(cmd, " "))
				if len(cmd) == 0 {
					return
				}
				handler := GetHandler(cmd[0])
				if handler == nil {
					return
				}
				stdout, err := beam.SendPipe(conn, data.Empty().Set("cmd", "log", "stdout").Set("fromcmd", cmd...).Bytes())
				if err != nil {
					return
				}
				defer stdout.Close()
				stderr, err := beam.SendPipe(conn, data.Empty().Set("cmd", "log", "stderr").Set("fromcmd", cmd...).Bytes())
				if err != nil {
					return
				}
				defer stderr.Close()
				Debugf("[handlers] calling %s\n", strings.Join(cmd, " "))
				handler(cmd, stdout, stderr, beam.Receiver(conn), beam.Sender(conn))
				Debugf("[handlers] returned: %s\n", strings.Join(cmd, " "))
			}()
			return nil
		})
		beam.Copy(r, priv)
		Debugf("[handlers] waiting for all tasks\n")
		tasks.Wait()
		Debugf("[handlers] all tasks returned\n")
	}()
	return pub, nil
}

func GetHandler(name string) Handler {
	if name == "logger" {
		return CmdLogger
	} else if name == "render" {
		return CmdRender
	} else if name == "devnull" {
		return CmdDevnull
	} else if name == "prompt" {
		return CmdPrompt
	} else if name == "stdio" {
		return CmdStdio
	} else if name == "echo" {
		return CmdEcho
	} else if name == "pass" {
		return CmdPass
	} else if name == "in" {
		return CmdIn
	} else if name == "exec" {
		return CmdExec
	} else if name == "trace" {
		return CmdTrace
	} else if name == "emit" {
		return CmdEmit
	} else if name == "print" {
		return CmdPrint
	} else if name == "multiprint" {
		return CmdMultiprint
	} else if name == "listen" {
		return CmdListen
	} else if name == "beamsend" {
		return CmdBeamsend
	} else if name == "beamreceive" {
		return CmdBeamreceive
	} else if name == "connect" {
		return CmdConnect
	} else if name == "openfile" {
		return CmdOpenfile
	} else if name == "spawn" {
		return CmdSpawn
	} else if name == "chdir" {
		return CmdChdir
	}
	return nil
}

// VARIOUS HELPER FUNCTIONS:

func connToFile(conn net.Conn) (f *os.File, err error) {
	if connWithFile, ok := conn.(interface {
		File() (*os.File, error)
	}); !ok {
		return nil, fmt.Errorf("no file descriptor available")
	} else {
		f, err = connWithFile.File()
		if err != nil {
			return nil, err
		}
	}
	return f, err
}

type Msg struct {
	payload    []byte
	attachment *os.File
}

func Logf(msg string, args ...interface{}) (int, error) {
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
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

func Fatalf(msg string, args ...interface{}) {
	Logf(msg, args...)
	os.Exit(1)
}

func Fatal(args ...interface{}) {
	Fatalf("%v", args[0])
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

func dialer(addr string) (chan net.Conn, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	connections := make(chan net.Conn)
	go func() {
		defer close(connections)
		for {
			conn, err := net.Dial(u.Scheme, u.Host)
			if err != nil {
				return
			}
			connections <- conn
		}
	}()
	return connections, nil
}

func listener(addr string) (chan net.Conn, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	l, err := net.Listen(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	connections := make(chan net.Conn)
	go func() {
		defer close(connections)
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			Logf("new connection\n")
			connections <- conn
		}
	}()
	return connections, nil
}

func SendToConn(connections chan net.Conn, src beam.Receiver) error {
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for {
		payload, attachment, err := src.Receive()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		conn, ok := <-connections
		if !ok {
			break
		}
		Logf("Sending %s\n", msgDesc(payload, attachment))
		tasks.Add(1)
		go func(payload []byte, attachment *os.File, conn net.Conn) {
			defer tasks.Done()
			if _, err := conn.Write([]byte(data.EncodeString(string(payload)))); err != nil {
				return
			}
			if attachment == nil {
				conn.Close()
				return
			}
			var iotasks sync.WaitGroup
			iotasks.Add(2)
			go func(attachment *os.File, conn net.Conn) {
				defer iotasks.Done()
				Debugf("copying the connection to [%d]\n", attachment.Fd())
				io.Copy(attachment, conn)
				attachment.Close()
				Debugf("done copying the connection to [%d]\n", attachment.Fd())
			}(attachment, conn)
			go func(attachment *os.File, conn net.Conn) {
				defer iotasks.Done()
				Debugf("copying [%d] to the connection\n", attachment.Fd())
				io.Copy(conn, attachment)
				conn.Close()
				Debugf("done copying [%d] to the connection\n", attachment.Fd())
			}(attachment, conn)
			iotasks.Wait()
		}(payload, attachment, conn)
	}
	return nil
}

func msgDesc(payload []byte, attachment *os.File) string {
	return beam.MsgDesc(payload, attachment)
}

func ReceiveFromConn(connections chan net.Conn, dst beam.Sender) error {
	for conn := range connections {
		err := func() error {
			Logf("parsing message from network...\n")
			defer Logf("done parsing message from network\n")
			buf := make([]byte, 4098)
			n, err := conn.Read(buf)
			if n == 0 {
				conn.Close()
				if err == io.EOF {
					return nil
				} else {
					return err
				}
			}
			Logf("decoding message from '%s'\n", buf[:n])
			header, skip, err := data.DecodeString(string(buf[:n]))
			if err != nil {
				conn.Close()
				return err
			}
			pub, priv, err := beam.SocketPair()
			if err != nil {
				return err
			}
			Logf("decoded message: %s\n", data.Message(header).Pretty())
			go func(skipped []byte, conn net.Conn, f *os.File) {
				// this closes both conn and f
				if len(skipped) > 0 {
					if _, err := f.Write(skipped); err != nil {
						Logf("ERROR: %v\n", err)
						f.Close()
						conn.Close()
						return
					}
				}
				bicopy(conn, f)
			}(buf[skip:n], conn, pub)
			if err := dst.Send([]byte(header), priv); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			Logf("Error reading from connection: %v\n", err)
		}
	}
	return nil
}

func bicopy(a, b io.ReadWriteCloser) {
	var iotasks sync.WaitGroup
	oneCopy := func(dst io.WriteCloser, src io.Reader) {
		defer iotasks.Done()
		io.Copy(dst, src)
		dst.Close()
	}
	iotasks.Add(2)
	go oneCopy(a, b)
	go oneCopy(b, a)
	iotasks.Wait()
}
