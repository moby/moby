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
)

// DockerCli represents the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	// initializing closure
	init func() error

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
func (cli *DockerCli) Initialize() error {
	if cli.init == nil {
		return nil
	}
	return cli.init()
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

// PsFormat returns the format string specified in the configuration.
// String contains columns and format specification, for example {{ID}\t{{Name}}.
func (cli *DockerCli) PsFormat() string {
	return cli.configFile.PsFormat
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

	cli.init = func() error {

		clientFlags.PostParse()

		hosts := clientFlags.Common.Hosts

		switch len(hosts) {
		case 0:
			hosts = []string{os.Getenv("DOCKER_HOST")}
		case 1:
			// only accept one host to talk to
		default:
			return errors.New("Please specify only one -H")
		}

		var e error
		if hosts[0], e = opts.ParseHost(hosts[0]); e != nil {
			return e
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

		configFile, e := cliconfig.Load(cliconfig.ConfigDir())
		if e != nil {
			fmt.Fprintf(cli.err, "WARNING: Error loading config file:%v\n", e)
		}
		cli.configFile = configFile

		return nil
	}

	return cli
}
