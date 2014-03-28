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
	if len(os.Args) == 1 {
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
					if err := executeScript(devnull, cmd); err != nil {
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
			script, err := dockerscript.Parse(os.Stdin)
			if err != nil {
				Fatal("parse error: %v\n", err)
			}
			if err := executeScript(devnull, script); err != nil {
				Fatal(err)
			}
		}
	} else {
		for _, scriptpath := range os.Args[1:] {
			f, err := os.Open(scriptpath)
			if err != nil {
				Fatal(err)
			}
			script, err := dockerscript.Parse(f)
			if err != nil {
				Fatal("parse error: %v\n", err)
			}
			if err := executeScript(devnull, script); err != nil {
				Fatal(err)
			}
		}
	}
}

func beamCopy(dst *net.UnixConn, src *net.UnixConn) (int, error) {
	var n int
	for {
		payload, attachment, err := beam.Receive(src)
		if err == io.EOF {
			return n, nil
		} else if err != nil {
			return n, err
		}
		if err := beam.Send(dst, payload, attachment); err != nil {
			if attachment != nil {
				attachment.Close()
			}
			return n, err
		}
		n++
	}
	panic("impossibru!")
	return n, nil
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
			_, attachment, err := beam.Receive(priv)
			if err != nil {
				return
			}
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
	var background sync.WaitGroup
	defer background.Wait()
	for _, cmd := range script {
		if cmd.Background {
			background.Add(1)
			go func(client *net.UnixConn, cmd *dockerscript.Command) {
				executeCommand(client, cmd)
				background.Done()
			}(client, cmd)
		} else {
			if err := executeCommand(client, cmd); err != nil {
				return err
			}
		}
	}
	return nil
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

func msgDesc(payload []byte, attachment *os.File) string {
	var filedesc string = "<nil>"
	if attachment != nil {
		filedesc = fmt.Sprintf("%d", attachment.Fd())
	}
	return fmt.Sprintf("'%s'[%s]", payload, filedesc)

}

func SendToConn(connections chan net.Conn, src *net.UnixConn) error {
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for {
		payload, attachment, err := beam.Receive(src)
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

func ReceiveFromConn(connections chan net.Conn, dst *net.UnixConn) error {
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
			if err := beam.Send(dst, []byte(header), priv); err != nil {
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
		n, err := beamCopy(client, outPub)
		if err != nil {
			Fatal(err)
		}
		Debugf("[%s] copied %d messages\n", strings.Join(cmd.Args, " "), n)
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
	if name == "pass" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			for {
				payload, attachment, err := beam.Receive(in)
				if err != nil {
					return
				}
				if err := beam.Send(out, payload, attachment); err != nil {
					if attachment != nil {
						attachment.Close()
					}
					return
				}
			}
		}
	} else if name == "in" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			os.Chdir(args[1])
			GetHandler("pass")([]string{"pass"}, in, out)
		}
	} else if name == "exec" {
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
			cmd.Stdin = os.Stdin
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
			outW.Close()
			errW.Close()
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
	} else if name == "beamsend" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			if len(args) < 2 {
				if err := beam.Send(out, data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil); err != nil {
					Fatal(err)
				}
				return
			}
			var connector func(string) (chan net.Conn, error)
			connector = dialer
			connections, err := connector(args[1])
			if err != nil {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			// Copy in to conn
			SendToConn(connections, in)
		}
	} else if name == "beamreceive" {
		return func(args []string, in *net.UnixConn, out *net.UnixConn) {
			if len(args) != 2 {
				if err := beam.Send(out, data.Empty().Set("status", "1").Set("message", "wrong number of arguments").Bytes(), nil); err != nil {
					Fatal(err)
				}
				return
			}
			var connector func(string) (chan net.Conn, error)
			connector = listener
			connections, err := connector(args[1])
			if err != nil {
				beam.Send(out, data.Empty().Set("status", "1").Set("message", err.Error()).Bytes(), nil)
				return
			}
			// Copy in to conn
			ReceiveFromConn(connections, out)
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
