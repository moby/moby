package client

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/homedir"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
)

// DockerCli represents the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string

	// configFile has the client configuration file
	configFile *cliconfig.ConfigFile
	// in holds the input stream and closer (io.ReadCloser) for the client.
	in io.ReadCloser
	// out holds the output stream (io.Writer) for the client.
	out io.Writer
	// err holds the error stream (io.Writer) for the client.
	err io.Writer
	// keyFile holds the key file as a string.
	keyFile string
	// tlsConfig holds the TLS configuration for the client, and will
	// set the scheme to https in NewDockerCli if present.
	tlsConfig *tls.Config
	// scheme holds the scheme of the client i.e. https.
	scheme string
	// inFd holds the file descriptor of the client's STDIN (if valid).
	inFd uintptr
	// outFd holds file descriptor of the client's STDOUT (if valid).
	outFd uintptr
	// isTerminalIn indicates whether the client's STDIN is a TTY
	isTerminalIn bool
	// isTerminalOut dindicates whether the client's STDOUT is a TTY
	isTerminalOut bool
	// transport holds the client transport instance.
	transport *http.Transport
}

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
}

func (cli *DockerCli) Out() io.Writer {
	return cli.out
}

func (cli *DockerCli) Err() io.Writer {
	return cli.err
}

func (cli *DockerCli) getMethod(args ...string) (func(...string) error, bool) {
	camelArgs := make([]string, len(args))
	for i, s := range args {
		if len(s) == 0 {
			return nil, false
		}
		camelArgs[i] = strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}
	methodName := "Cmd" + strings.Join(camelArgs, "")
	method := reflect.ValueOf(cli).MethodByName(methodName)
	if !method.IsValid() {
		return nil, false
	}
	return method.Interface().(func(...string) error), true
}

// Cmd executes the specified command.
func (cli *DockerCli) Cmd(args ...string) error {
	if len(args) > 1 {
		method, exists := cli.getMethod(args[:2]...)
		if exists {
			return method(args[2:]...)
		}
	}
	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			return fmt.Errorf("docker: '%s' is not a docker command.\nSee 'docker --help'.", args[0])
		}
		return method(args[1:]...)
	}
	return cli.CmdHelp()
}

// Subcmd is a subcommand of the main "docker" command.
// A subcommand represents an action that can be performed
// from the Docker command line client.
//
// To see all available subcommands, run "docker --help".
func (cli *DockerCli) Subcmd(name, signature, description string, exitOnError bool) *flag.FlagSet {
	var errorHandling flag.ErrorHandling
	if exitOnError {
		errorHandling = flag.ExitOnError
	} else {
		errorHandling = flag.ContinueOnError
	}
	flags := flag.NewFlagSet(name, errorHandling)
	if signature != "" {
		signature = " " + signature
	}
	flags.Usage = func() {
		flags.ShortUsage()
		flags.PrintDefaults()
	}
	flags.ShortUsage = func() {
		options := ""
		if flags.FlagCountUndeprecated() > 0 {
			options = " [OPTIONS]"
		}
		fmt.Fprintf(flags.Out(), "\nUsage: docker %s%s%s\n\n%s\n", name, options, signature, description)
	}
	return flags
}

// CheckTtyInput checks if we are trying to attach to a container tty
// from a non-tty client input stream, and if so, returns an error.
func (cli *DockerCli) CheckTtyInput(attachStdin, ttyMode bool) error {
	// In order to attach to a container tty, input stream for the client must
	// be a tty itself: redirecting or piping the client standard input is
	// incompatible with `docker run -t`, `docker exec -t` or `docker attach`.
	if ttyMode && attachStdin && !cli.isTerminalIn {
		return errors.New("cannot enable tty mode on non tty input")
	}
	return nil
}

// NewDockerCli returns a DockerCli instance with IO output and error streams set by in, out and err.
// The key file, protocol (i.e. unix) and address are passed in as strings, along with the tls.Config. If the tls.Config
// is set the client scheme will be set to https.
// The client will be given a 32-second timeout (see https://github.com/docker/docker/pull/8035).
func NewDockerCli(in io.ReadCloser, out, err io.Writer, keyFile string, proto, addr string, tlsConfig *tls.Config) *DockerCli {
	var (
		inFd          uintptr
		outFd         uintptr
		isTerminalIn  = false
		isTerminalOut = false
		scheme        = "http"
	)

	if tlsConfig != nil {
		scheme = "https"
	}
	if in != nil {
		inFd, isTerminalIn = term.GetFdInfo(in)
	}

	if out != nil {
		outFd, isTerminalOut = term.GetFdInfo(out)
	}

	if err == nil {
		err = out
	}

	// The transport is created here for reuse during the client session.
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	utils.ConfigureTCPTransport(tr, proto, addr)

	configFile, e := cliconfig.Load(filepath.Join(homedir.Get(), ".docker"))
	if e != nil {
		fmt.Fprintf(err, "WARNING: Error loading config file:%v\n", e)
	}

	return &DockerCli{
		proto:         proto,
		addr:          addr,
		configFile:    configFile,
		in:            in,
		out:           out,
		err:           err,
		keyFile:       keyFile,
		inFd:          inFd,
		outFd:         outFd,
		isTerminalIn:  isTerminalIn,
		isTerminalOut: isTerminalOut,
		tlsConfig:     tlsConfig,
		scheme:        scheme,
		transport:     tr,
	}
}
