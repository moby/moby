package main

import (
	"bufio"
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
	"text/template"
	"flag"
)

var rootPlugins = []string{
	"stdio",
}

var (
	flX bool
)

func main() {
	flag.BoolVar(&flX, "x", false, "print commands as they are being executed")
	flag.Parse()
	if flag.NArg() == 0{
		if term.IsTerminal(0) {
			// No arguments, stdin is terminal --> interactive mode
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
	handlers, err := Handlers()
	if err != nil {
		return err
	}
	defer handlers.Close()
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
	job, err := beam.SendConn(out, data.Empty().Set("cmd", cmd.Args...).Bytes())
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


type Handler func([]string, beam.Receiver, beam.Sender)


func Handlers() (*beam.UnixConn, error) {
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
		for {
			Debugf("[handlers] waiting for next job...\n")
			payload, conn, err := beam.ReceiveConn(priv)
			Debugf("[handlers] ReceiveConn() returned %v\n", err)
			if err != nil {
				return
			}
			tasks.Add(1)
			go func(payload []byte, conn *beam.UnixConn) {
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
				Debugf("[handlers] calling %s\n", strings.Join(cmd, " "))
				handler(cmd, beam.Receiver(conn), beam.Sender(conn))
				Debugf("[handlers] returned: %s\n", strings.Join(cmd, " "))
			}(payload, conn)
		}
		Debugf("[handlers] waiting for all tasks\n")
		tasks.Wait()
		Debugf("[handlers] all tasks returned\n")
	}()
	return pub, nil
}

func GetHandler(name string) Handler {
	if name == "logger" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			var tasks sync.WaitGroup
			stdout, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stdout").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stdout.Close()
			stderr, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stderr").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stderr.Close()
			if err := os.MkdirAll("logs", 0700); err != nil {
				fmt.Fprintf(stderr, "%v\n", err)
				return
			}
			var n int = 1
			for {
				payload, attachment, err := in.Receive()
				if err != nil {
					return
				}
				if attachment == nil {
					continue
				}
				w, err := beam.SendPipe(out, payload)
				if err != nil {
					fmt.Fprintf(stderr, "%v\n", err)
					attachment.Close()
					return
				}
				tasks.Add(1)
				go func(payload []byte, attachment *os.File, n int, sink *os.File) {
					defer tasks.Done()
					defer attachment.Close()
					defer sink.Close()
					cmd := data.Message(payload).Get("cmd")
					if cmd == nil || len(cmd) == 0 {
						return
					}
					if cmd[0] != "log" {
						return
					}
					var streamname string
					if len(cmd) == 1 || cmd[1] == "stdout" {
						streamname = "stdout"
					} else {
						streamname = cmd[1]
					}
					if fromcmd := data.Message(payload).Get("fromcmd"); len(fromcmd) != 0 {
						streamname = fmt.Sprintf("%s-%s", strings.Replace(strings.Join(fromcmd, "_"), "/", "_", -1), streamname)
					}
					logfile, err := os.OpenFile(path.Join("logs", fmt.Sprintf("%d-%s", n, streamname)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0700)
					if err != nil {
						fmt.Fprintf(stderr, "%v\n", err)
						return
					}
					io.Copy(io.MultiWriter(logfile, sink), attachment)
					logfile.Sync()
					logfile.Close()
				}(payload, attachment, n, w)
				n++
			}
			tasks.Wait()
		}
	} else if name == "render" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			stdout, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stdout").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stdout.Close()
			stderr, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stderr").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stderr.Close()
			if len(args) != 2 {
				fmt.Fprintf(stderr, "Usage: %s FORMAT\n", args[0])
				out.Send(data.Empty().Set("status", "1").Bytes(), nil)
				return
			}
			txt := args[1]
			if !strings.HasSuffix(txt, "\n") {
				txt += "\n"
			}
			t := template.Must(template.New("render").Parse(txt))
			for {
				payload, attachment, err := in.Receive()
				if err != nil {
					return
				}
				msg, err := data.Decode(string(payload))
				if err != nil {
					fmt.Fprintf(stderr, "decode error: %v\n")
				}
				if err := t.Execute(stdout, msg); err != nil {
					fmt.Fprintf(stderr, "rendering error: %v\n", err)
					out.Send(data.Empty().Set("status", "1").Bytes(), nil)
					return
				}
				if err := out.Send(payload, attachment); err != nil {
					return
				}
			}
		}
	} else if name == "devnull" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			for {
				_, attachment, err := in.Receive()
				if err != nil {
					return
				}
				if attachment != nil {
					attachment.Close()
				}
			}
		}
	} else if name == "prompt" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			stdout, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stdout").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stdout.Close()
			stderr, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stderr").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stderr.Close()
			if len(args) < 2 {
				fmt.Fprintf(stderr, "usage: %s PROMPT...\n", args[0])
				return
			}
			if !term.IsTerminal(0) {
				fmt.Fprintf(stderr, "can't prompt: no tty available...\n")
				return
			}
			fmt.Printf("%s: ", strings.Join(args[1:], " "))
			oldState, _ := term.SaveState(0)
			term.DisableEcho(0, oldState)
			line, _, err := bufio.NewReader(os.Stdin).ReadLine()
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return
			}
			val := string(line)
			fmt.Printf("\n")
			term.RestoreTerminal(0, oldState)
			out.Send(data.Empty().Set("fromcmd", args...).Set("value", val).Bytes(), nil)
		}
	} else if name == "stdio" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			stdout, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stdout").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stdout.Close()
			stderr, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stderr").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stderr.Close()
			var tasks sync.WaitGroup
			defer tasks.Wait()

			r := beam.NewRouter(out)
			r.NewRoute().HasAttachment().KeyStartsWith("cmd", "log").Handler(func(payload []byte, attachment *os.File) error {
				tasks.Add(1)
				go func() {
					defer tasks.Done()
					defer attachment.Close()
					io.Copy(os.Stdout, attachment)
					attachment.Close()
				}()
				return nil
			}).Tee(out)

			if _, err := beam.Copy(r, in); err != nil {
				Fatal(err)
				fmt.Fprintf(stderr, "%v\n", err)
				return
			}
		}
	} else if name == "echo" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			stdout, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stdout").Bytes())
			if err != nil {
				return
			}
			fmt.Fprintln(stdout, strings.Join(args[1:], " "))
			stdout.Close()
		}
	} else if name == "pass" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			for {
				payload, attachment, err := in.Receive()
				if err != nil {
					return
				}
				if err := out.Send(payload, attachment); err != nil {
					if attachment != nil {
						attachment.Close()
					}
					return
				}
			}
		}
	} else if name == "in" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			os.Chdir(args[1])
			GetHandler("pass")([]string{"pass"}, in, out)
		}
	} else if name == "exec" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			cmd := exec.Command(args[1], args[2:]...)
			stdout, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stdout").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stdout.Close()
			cmd.Stdout = stdout
			stderr, err := beam.SendPipe(out, data.Empty().Set("cmd", "log", "stderr").Set("fromcmd", args...).Bytes())
			if err != nil {
				return
			}
			defer stderr.Close()
			cmd.Stderr = stderr
			cmd.Stdin = os.Stdin
			execErr := cmd.Run()
			var status string
			if execErr != nil {
				status = execErr.Error()
			} else {
				status = "ok"
			}
			out.Send(data.Empty().Set("status", status).Set("cmd", args...).Bytes(), nil)
		}
	} else if name == "trace" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			for {
				p, a, err := in.Receive()
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
				out.Send(p, a)
			}
		}
	} else if name == "emit" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			out.Send(data.Parse(args[1:]).Bytes(), nil)
		}
	} else if name == "print" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			for {
				payload, a, err := in.Receive()
				if err != nil {
					return
				}
				// Skip commands
				if a != nil && data.Message(payload).Get("cmd") == nil {
					dup, err := beam.SendPipe(out, payload)
					if err != nil {
						a.Close()
						return
					}
					io.Copy(io.MultiWriter(os.Stdout, dup), a)
					dup.Close()
				} else {
					if err := out.Send(payload, a); err != nil {
						return
					}
				}
			}
		}
	} else if name == "multiprint" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			var tasks sync.WaitGroup
			for {
				payload, a, err := in.Receive()
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
		return func(args []string, in beam.Receiver, out beam.Sender) {
			if len(args) != 2 {
				out.Send(data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil)
				return
			}
			u, err := url.Parse(args[1])
			if err != nil {
				out.Send(data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			l, err := net.Listen(u.Scheme, u.Host)
			if err != nil {
				out.Send(data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			for {
				conn, err := l.Accept()
				if err != nil {
					out.Send(data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
					return
				}
				f, err := connToFile(conn)
				if err != nil {
					conn.Close()
					continue
				}
				out.Send(data.Empty().Set("type", "socket").Set("remoteaddr", conn.RemoteAddr().String()).Bytes(), f)
			}
		}
	} else if name == "beamsend" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			if len(args) < 2 {
				if err := out.Send(data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil); err != nil {
					Fatal(err)
				}
				return
			}
			var connector func(string) (chan net.Conn, error)
			connector = dialer
			connections, err := connector(args[1])
			if err != nil {
				out.Send(data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			// Copy in to conn
			SendToConn(connections, in)
		}
	} else if name == "beamreceive" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			if len(args) != 2 {
				if err := out.Send(data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil); err != nil {
					Fatal(err)
				}
				return
			}
			var connector func(string) (chan net.Conn, error)
			connector = listener
			connections, err := connector(args[1])
			if err != nil {
				out.Send(data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			// Copy in to conn
			ReceiveFromConn(connections, out)
		}
	} else if name == "connect" {
		return func(args []string, in beam.Receiver, out beam.Sender) {
			if len(args) != 2 {
				out.Send(data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil)
				return
			}
			u, err := url.Parse(args[1])
			if err != nil {
				out.Send(data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			var tasks sync.WaitGroup
			for {
				_, attachment, err := in.Receive()
				if err != nil {
					break
				}
				if attachment == nil {
					continue
				}
				Logf("connecting to %s/%s\n", u.Scheme, u.Host)
				conn, err := net.Dial(u.Scheme, u.Host)
				if err != nil {
					out.Send(data.Empty().Set("cmd", "msg", "connect error: " + err.Error()).Bytes(), nil)
					return
				}
				out.Send(data.Empty().Set("cmd", "msg", "connection established").Bytes(), nil)
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
		return func(args []string, in beam.Receiver, out beam.Sender) {
			for _, name := range args {
				f, err := os.Open(name)
				if err != nil {
					continue
				}
				if err := out.Send(data.Empty().Set("path", name).Set("type", "file").Bytes(), f); err != nil {
					f.Close()
				}
			}
		}
	}
	return nil
}


// VARIOUS HELPER FUNCTIONS:

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
			connections <-conn
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
			connections<-conn
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
		err := func () error {
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

