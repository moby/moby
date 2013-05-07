package rcli

// rcli (Remote Command-Line Interface) is a simple protocol for...
// serving command-line interfaces remotely.
//
// rcli can be used over any transport capable of a) sending binary streams in
// both directions, and b) capable of half-closing a connection. TCP and Unix sockets
// are the usual suspects.

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker/term"
	"io"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
)

type DockerConnOptions struct {
	RawTerminal bool
}

type DockerConn interface {
	io.ReadWriteCloser
	CloseWrite() error
	CloseRead() error
	GetOptions() *DockerConnOptions
	SetOptionRawTerminal()
	Flush() error
}

type DockerLocalConn struct {
	writer     io.WriteCloser
	savedState *term.State
}

func NewDockerLocalConn(w io.WriteCloser) *DockerLocalConn {
	return &DockerLocalConn{
		writer: w,
	}
}

func (c *DockerLocalConn) Read(b []byte) (int, error) {
	return 0, fmt.Errorf("DockerLocalConn does not implement Read()")
}

func (c *DockerLocalConn) Write(b []byte) (int, error) { return c.writer.Write(b) }

func (c *DockerLocalConn) Close() error {
	if c.savedState != nil {
		RestoreTerminal(c.savedState)
		c.savedState = nil
	}
	return c.writer.Close()
}

func (c *DockerLocalConn) Flush() error { return nil }

func (c *DockerLocalConn) CloseWrite() error { return nil }

func (c *DockerLocalConn) CloseRead() error { return nil }

func (c *DockerLocalConn) GetOptions() *DockerConnOptions { return nil }

func (c *DockerLocalConn) SetOptionRawTerminal() {
	if state, err := SetRawTerminal(); err != nil {
		if os.Getenv("DEBUG") != "" {
			log.Printf("Can't set the terminal in raw mode: %s", err)
		}
	} else {
		c.savedState = state
	}
}

var UnknownDockerProto = fmt.Errorf("Only TCP is actually supported by Docker at the moment")

func dialDocker(proto string, addr string) (DockerConn, error) {
	conn, err := net.Dial(proto, addr)
	if err != nil {
		return nil, err
	}
	switch i := conn.(type) {
	case *net.TCPConn:
		return NewDockerTCPConn(i, true), nil
	}
	return nil, UnknownDockerProto
}

func newDockerFromConn(conn net.Conn, client bool) (DockerConn, error) {
	switch i := conn.(type) {
	case *net.TCPConn:
		return NewDockerTCPConn(i, client), nil
	}
	return nil, UnknownDockerProto
}

func newDockerServerConn(conn net.Conn) (DockerConn, error) {
	return newDockerFromConn(conn, false)
}

type Service interface {
	Name() string
	Help() string
}

type Cmd func(io.ReadCloser, io.Writer, ...string) error
type CmdMethod func(Service, io.ReadCloser, io.Writer, ...string) error

// FIXME: For reverse compatibility
func call(service Service, stdin io.ReadCloser, stdout DockerConn, args ...string) error {
	return LocalCall(service, stdin, stdout, args...)
}

func LocalCall(service Service, stdin io.ReadCloser, stdout DockerConn, args ...string) error {
	if len(args) == 0 {
		args = []string{"help"}
	}
	flags := flag.NewFlagSet("main", flag.ContinueOnError)
	flags.SetOutput(stdout)
	flags.Usage = func() { stdout.Write([]byte(service.Help())) }
	if err := flags.Parse(args); err != nil {
		return err
	}
	cmd := flags.Arg(0)
	log.Printf("%s\n", strings.Join(append(append([]string{service.Name()}, cmd), flags.Args()[1:]...), " "))
	if cmd == "" {
		cmd = "help"
	}
	method := getMethod(service, cmd)
	if method != nil {
		return method(stdin, stdout, flags.Args()[1:]...)
	}
	return fmt.Errorf("No such command: %s", cmd)
}

func getMethod(service Service, name string) Cmd {
	if name == "help" {
		return func(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
			if len(args) == 0 {
				stdout.Write([]byte(service.Help()))
			} else {
				if method := getMethod(service, args[0]); method == nil {
					return fmt.Errorf("No such command: %s", args[0])
				} else {
					method(stdin, stdout, "--help")
				}
			}
			return nil
		}
	}
	methodName := "Cmd" + strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	method, exists := reflect.TypeOf(service).MethodByName(methodName)
	if !exists {
		return nil
	}
	return func(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
		ret := method.Func.CallSlice([]reflect.Value{
			reflect.ValueOf(service),
			reflect.ValueOf(stdin),
			reflect.ValueOf(stdout),
			reflect.ValueOf(args),
		})[0].Interface()
		if ret == nil {
			return nil
		}
		return ret.(error)
	}
}

func Subcmd(output io.Writer, name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintf(output, "\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}
