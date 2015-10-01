package client

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/sockets"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/pkg/tlsconfig"
	"os/exec"
)

// DockerCli represents the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	// initializing closure
	init func() error

	// initializing closure for config file
	initConfig func()

	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string
	// basePath holds the path to prepend to the requests
	basePath string

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
	// isTerminalOut indicates whether the client's STDOUT is a TTY
	isTerminalOut bool
	// transport holds the client transport instance.
	transport *http.Transport
}

// Initialize calls the init function that will setup the configuration for the client
// such as the TLS, tcp and other parameters used to run the client.
func (dockerCli *DockerCli) Initialize() error {
	if dockerCli.init == nil {
		return nil
	}
	return dockerCli.init()
}

// CheckTtyInput checks if we are trying to attach to a container tty
// from a non-tty client input stream, and if so, returns an error.
func (dockerCli *DockerCli) CheckTtyInput(attachStdin, ttyMode bool) error {
	// In order to attach to a container tty, input stream for the client must
	// be a tty itself: redirecting or piping the client standard input is
	// incompatible with `docker run -t`, `docker exec -t` or `docker attach`.
	if ttyMode && attachStdin && !dockerCli.isTerminalIn {
		return errors.New("cannot enable tty mode on non tty input")
	}
	return nil
}

// PsFormat returns the format string specified in the configuration.
// String contains columns and format specification, for example {{ID}\t{{Name}}.
func (dockerCli *DockerCli) PsFormat() string {
	return dockerCli.configFile.PsFormat
}

// NewDockerCli returns a DockerCli instance with IO output and error streams set by in, out and err.
// The key file, protocol (i.e. unix) and address are passed in as strings, along with the tls.Config. If the tls.Config
// is set the client scheme will be set to https.
// The client will be given a 32-second timeout (see https://github.com/docker/docker/pull/8035).
func NewDockerCli(in io.ReadCloser, out, err io.Writer, clientFlags *cli.ClientFlags) *DockerCli {
	cli := &DockerCli{
		in:      in,
		out:     out,
		err:     err,
		keyFile: clientFlags.Common.TrustKey,
	}

	cli.initConfig = func() {
		clientFlags.PostParse()
		configFile, e := cliconfig.Load(cliconfig.ConfigDir())
		if e != nil {
			fmt.Fprintf(cli.err, "WARNING: Error loading config file:%v\n", e)
		}
		cli.configFile = configFile
	}

	cli.init = func() error {

		// probably unnecessary since ResolveAlias is always called
		if cli.configFile == nil {
			cli.initConfig()
		}

		hosts := clientFlags.Common.Hosts

		switch len(hosts) {
		case 0:
			defaultHost := os.Getenv("DOCKER_HOST")
			if defaultHost == "" {
				defaultHost = opts.DefaultHost
			}
			defaultHost, err := opts.ValidateHost(defaultHost)
			if err != nil {
				return err
			}
			hosts = []string{defaultHost}
		case 1:
		// only accept one host to talk to
		default:
			return errors.New("Please specify only one -H")
		}

		protoAddrParts := strings.SplitN(hosts[0], "://", 2)
		cli.proto, cli.addr = protoAddrParts[0], protoAddrParts[1]

		if cli.proto == "tcp" {
			// error is checked in pkg/parsers already
			parsed, _ := url.Parse("tcp://" + cli.addr)
			cli.addr = parsed.Host
			cli.basePath = parsed.Path
		}

		if clientFlags.Common.TLSOptions != nil {
			cli.scheme = "https"
			var e error
			cli.tlsConfig, e = tlsconfig.Client(*clientFlags.Common.TLSOptions)
			if e != nil {
				return e
			}
		} else {
			cli.scheme = "http"
		}

		if cli.in != nil {
			cli.inFd, cli.isTerminalIn = term.GetFdInfo(cli.in)
		}
		if cli.out != nil {
			cli.outFd, cli.isTerminalOut = term.GetFdInfo(cli.out)
		}

		// The transport is created here for reuse during the client session.
		cli.transport = &http.Transport{
			TLSClientConfig: cli.tlsConfig,
		}
		sockets.ConfigureTCPTransport(cli.transport, cli.proto, cli.addr)

		return nil
	}

	return cli
}

// ResolveAlias tries to match name with an alias
// if there is a match, it returns an Alias object
func (dockerCli *DockerCli) ResolveAlias(aliasName string) (alias cli.Alias, aliasResolved bool) {
	dockerCli.initConfig()
	if aliasCmd, exists := dockerCli.configFile.Aliases[aliasName]; exists {
		if isComplexAlias(aliasCmd[0]) {
			// if cmd starts with a ! it's a complex alias
			return &cli.ComplexAlias{SimpleAlias: cli.SimpleAlias{Name: aliasName, Cmd: aliasCmd}, CmdExecutor: dockerCli.ComplexAliasExecutor}, true
		}
		//simple case
		return &cli.SimpleAlias{Name: aliasName, Cmd: aliasCmd}, true
	}
	return nil, false
}

// ComplexAliasExecutor executes the given aliasCommand in a shell
func (dockerCli *DockerCli) ComplexAliasExecutor(aliasCommand []string) error {
	// strip leading '!'
	aliasCommand[0] = aliasCommand[0][1:]
	cmdString := strings.Join(aliasCommand, " ")

	cmd := exec.Command("sh", "-c", cmdString)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
