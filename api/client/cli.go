package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/template"

	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/registry"
)

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
}

func toCamel(str string) string {
	return strings.ToUpper(str[:1]) + strings.ToLower(str[1:])
}

func (cli *DockerCli) getMethod(args []string) (func(...string) error, int, error) {
	name := args[0]
	if name == "" {
		return nil, 0, fmt.Errorf("Command not found")
	}

	methodName := "Cmd" + toCamel(name)
	method := reflect.ValueOf(cli).MethodByName(methodName)
	if !method.IsValid() {
		return nil, 0, fmt.Errorf("Command not found: %s", name)
	}

	numArgs := method.Type().NumIn()
	if numArgs == 1 {
		// Leaf command
		return method.Interface().(func(...string) error), 1, nil
	}

	// command with subcommands
	f := method.Interface().(func(bool, ...string) error)

	if len(args) == 1 || args[1] == "" || strings.HasPrefix(args[1], "-") {
		// No subcommand, call directly
		return func(args ...string) error { return f(false, args...) }, 1, nil
	}

	subName := args[1]

	methodName = "Cmd" + toCamel(name) + toCamel(subName)
	method = reflect.ValueOf(cli).MethodByName(methodName)
	if !method.IsValid() {
		return nil, 1, fmt.Errorf("Command not found: %s %s", name, subName)
	}

	return method.Interface().(func(...string) error), 2, nil
}

func (cli *DockerCli) ParseCommands(args ...string) error {
	if len(args) > 0 {
		method, consumed, err := cli.getMethod(args)
		if err != nil {
			fmt.Println("Error: ", err)
			return cli.CmdHelp(args[0:consumed]...)
		}
		return method(args[consumed:]...)
	}
	return cli.CmdHelp()
}

func (cli *DockerCli) Subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintf(cli.err, "\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
		os.Exit(2)
	}
	return flags
}

func (cli *DockerCli) LoadConfigFile() (err error) {
	cli.configFile, err = registry.LoadConfig(os.Getenv("HOME"))
	if err != nil {
		fmt.Fprintf(cli.err, "WARNING: %s\n", err)
	}
	return err
}

func NewDockerCli(in io.ReadCloser, out, err io.Writer, proto, addr string, tlsConfig *tls.Config) *DockerCli {
	var (
		isTerminal = false
		terminalFd uintptr
		scheme     = "http"
	)

	if tlsConfig != nil {
		scheme = "https"
	}

	if in != nil {
		if file, ok := out.(*os.File); ok {
			terminalFd = file.Fd()
			isTerminal = term.IsTerminal(terminalFd)
		}
	}

	if err == nil {
		err = out
	}
	return &DockerCli{
		proto:      proto,
		addr:       addr,
		in:         in,
		out:        out,
		err:        err,
		isTerminal: isTerminal,
		terminalFd: terminalFd,
		tlsConfig:  tlsConfig,
		scheme:     scheme,
	}
}

type DockerCli struct {
	proto      string
	addr       string
	configFile *registry.ConfigFile
	in         io.ReadCloser
	out        io.Writer
	err        io.Writer
	isTerminal bool
	terminalFd uintptr
	tlsConfig  *tls.Config
	scheme     string
}
