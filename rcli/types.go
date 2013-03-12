package rcli

// rcli (Remote Command-Line Interface) is a simple protocol for...
// serving command-line interfaces remotely.
//
// rcli can be used over any transport capable of a) sending binary streams in
// both directions, and b) capable of half-closing a connection. TCP and Unix sockets
// are the usual suspects.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"
)

type Service interface {
	Name() string
	Help() string
}

type Cmd func(io.ReadCloser, io.Writer, ...string) error
type CmdMethod func(Service, io.ReadCloser, io.Writer, ...string) error

func call(service Service, stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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
	return errors.New("No such command: " + cmd)
}

func getMethod(service Service, name string) Cmd {
	if name == "help" {
		return func(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
			if len(args) == 0 {
				stdout.Write([]byte(service.Help()))
			} else {
				if method := getMethod(service, args[0]); method == nil {
					return errors.New("No such command: " + args[0])
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
