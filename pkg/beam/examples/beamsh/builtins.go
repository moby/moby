package main

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/beam/data"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/utils"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"text/template"
)

func CmdLogger(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	if err := os.MkdirAll("logs", 0700); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
	var tasks sync.WaitGroup
	defer tasks.Wait()
	var n int = 1
	r := beam.NewRouter(out)
	r.NewRoute().HasAttachment().KeyStartsWith("cmd", "log").Handler(func(payload []byte, attachment *os.File) error {
		tasks.Add(1)
		go func(n int) {
			defer tasks.Done()
			defer attachment.Close()
			var streamname string
			if cmd := data.Message(payload).Get("cmd"); len(cmd) == 1 || cmd[1] == "stdout" {
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
			defer logfile.Close()
			io.Copy(logfile, attachment)
			logfile.Sync()
		}(n)
		n++
		return nil
	}).Tee(out)
	if _, err := beam.Copy(r, in); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
}

func CmdRender(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdDevnull(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdPrompt(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdStdio(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdEcho(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	fmt.Fprintln(stdout, strings.Join(args[1:], " "))
}

func CmdPass(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdSpawn(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	c := exec.Command(utils.SelfPath())
	r, w, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
	c.Stdin = r
	c.Stdout = stdout
	c.Stderr = stderr
	go func() {
		fmt.Fprintf(w, strings.Join(args[1:], " "))
		w.Sync()
		w.Close()
	}()
	if err := c.Run(); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
}

func CmdIn(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	os.Chdir(args[1])
	GetHandler("pass")([]string{"pass"}, stdout, stderr, in, out)
}

func CmdExec(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	cmd := exec.Command(args[1], args[2:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	//cmd.Stdin = os.Stdin
	local, remote, err := beam.SocketPair()
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
	child, err := beam.FileConn(local)
	if err != nil {
		local.Close()
		remote.Close()
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
	local.Close()
	cmd.ExtraFiles = append(cmd.ExtraFiles, remote)

	var tasks sync.WaitGroup
	tasks.Add(1)
	go func() {
		defer Debugf("done copying to child\n")
		defer tasks.Done()
		defer child.CloseWrite()
		beam.Copy(child, in)
	}()

	tasks.Add(1)
	go func() {
		defer Debugf("done copying from child %d\n")
		defer tasks.Done()
		r := beam.NewRouter(out)
		r.NewRoute().All().Handler(func(p []byte, a *os.File) error {
			return out.Send(data.Message(p).Set("pid", fmt.Sprintf("%d", cmd.Process.Pid)).Bytes(), a)
		})
		beam.Copy(r, child)
	}()
	execErr := cmd.Run()
	// We can close both ends of the socket without worrying about data stuck in the buffer,
	// because unix socket writes are fully synchronous.
	child.Close()
	tasks.Wait()
	var status string
	if execErr != nil {
		status = execErr.Error()
	} else {
		status = "ok"
	}
	out.Send(data.Empty().Set("status", status).Set("cmd", args...).Bytes(), nil)
}

func CmdTrace(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	r := beam.NewRouter(out)
	r.NewRoute().All().Handler(func(payload []byte, attachment *os.File) error {
		var sfd string = "nil"
		if attachment != nil {
			sfd = fmt.Sprintf("%d", attachment.Fd())
		}
		fmt.Printf("===> %s [%s]\n", data.Message(payload).Pretty(), sfd)
		out.Send(payload, attachment)
		return nil
	})
	beam.Copy(r, in)
}

func CmdEmit(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	out.Send(data.Parse(args[1:]).Bytes(), nil)
}

func CmdPrint(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdMultiprint(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	var tasks sync.WaitGroup
	defer tasks.Wait()
	r := beam.NewRouter(out)
	multiprint := func(p []byte, a *os.File) error {
		tasks.Add(1)
		go func() {
			defer tasks.Done()
			defer a.Close()
			msg := data.Message(string(p))
			input := bufio.NewScanner(a)
			for input.Scan() {
				fmt.Printf("[%s] %s\n", msg.Pretty(), input.Text())
			}
		}()
		return nil
	}
	r.NewRoute().KeyIncludes("type", "job").Passthrough(out)
	r.NewRoute().HasAttachment().Handler(multiprint).Tee(out)
	beam.Copy(r, in)
}

func CmdListen(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdBeamsend(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdBeamreceive(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdConnect(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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
			out.Send(data.Empty().Set("cmd", "msg", "connect error: "+err.Error()).Bytes(), nil)
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

func CmdOpenfile(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
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

func CmdChdir(args []string, stdout, stderr io.Writer, in beam.Receiver, out beam.Sender) {
	os.Chdir(args[1])
}
