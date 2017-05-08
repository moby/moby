package client

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/api"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/docker/cliconfig/credentials"
	"github.com/docker/docker/dockerversion"
	dopts "github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
)

// DockerCli represents the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	// initializing closure
	init func() error

	// configFile has the client configuration file
	configFile *configfile.ConfigFile
	// in holds the input stream and closer (io.ReadCloser) for the client.
	in io.ReadCloser
	// out holds the output stream (io.Writer) for the client.
	out io.Writer
	// err holds the error stream (io.Writer) for the client.
	err io.Writer
	// keyFile holds the key file as a string.
	keyFile string
	// inFd holds the file descriptor of the client's STDIN (if valid).
	inFd uintptr
	// outFd holds file descriptor of the client's STDOUT (if valid).
	outFd uintptr
	// isTerminalIn indicates whether the client's STDIN is a TTY
	isTerminalIn bool
	// isTerminalOut indicates whether the client's STDOUT is a TTY
	isTerminalOut bool
	// client is the http client that performs all API operations
	client client.APIClient
	// inState holds the terminal input state
	inState *term.State
	// outState holds the terminal output state
	outState *term.State
}

// Client returns the APIClient
func (cli *DockerCli) Client() client.APIClient {
	return cli.client
}

// Out returns the writer used for stdout
func (cli *DockerCli) Out() io.Writer {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *DockerCli) Err() io.Writer {
	return cli.err
}

// In returns the reader used for stdin
func (cli *DockerCli) In() io.ReadCloser {
	return cli.in
}

// ConfigFile returns the ConfigFile
func (cli *DockerCli) ConfigFile() *configfile.ConfigFile {
	return cli.configFile
}

// IsTerminalIn returns true if the clients stdin is a TTY
func (cli *DockerCli) IsTerminalIn() bool {
	return cli.isTerminalIn
}

// IsTerminalOut returns true if the clients stdout is a TTY
func (cli *DockerCli) IsTerminalOut() bool {
	return cli.isTerminalOut
}

// OutFd returns the fd for the stdout stream
func (cli *DockerCli) OutFd() uintptr {
	return cli.outFd
}

// CheckTtyInput checks if we are trying to attach to a container tty
// from a non-tty client input stream, and if so, returns an error.
func (cli *DockerCli) CheckTtyInput(attachStdin, ttyMode bool) error {
	// In order to attach to a container tty, input stream for the client must
	// be a tty itself: redirecting or piping the client standard input is
	// incompatible with `docker run -t`, `docker exec -t` or `docker attach`.
	if ttyMode && attachStdin && !cli.isTerminalIn {
		eText := "the input device is not a TTY"
		if runtime.GOOS == "windows" {
			return errors.New(eText + ".  If you are using mintty, try prefixing the command with 'winpty'")
		}
		return errors.New(eText)
	}
	return nil
}

func (cli *DockerCli) setRawTerminal() error {
	if os.Getenv("NORAW") == "" {
		if cli.isTerminalIn {
			state, err := term.SetRawTerminal(cli.inFd)
			if err != nil {
				return err
			}
			cli.inState = state
		}
		if cli.isTerminalOut {
			state, err := term.SetRawTerminalOutput(cli.outFd)
			if err != nil {
				return err
			}
			cli.outState = state
		}
	}
	return nil
}

func (cli *DockerCli) restoreTerminal(in io.Closer) error {
	if cli.inState != nil {
		term.RestoreTerminal(cli.inFd, cli.inState)
	}
	if cli.outState != nil {
		term.RestoreTerminal(cli.outFd, cli.outState)
	}
	// WARNING: DO NOT REMOVE THE OS CHECK !!!
	// For some reason this Close call blocks on darwin..
	// As the client exists right after, simply discard the close
	// until we find a better solution.
	if in != nil && runtime.GOOS != "darwin" {
		return in.Close()
	}
	return nil
}

// Initialize the dockerCli runs initialization that must happen after command
// line flags are parsed.
func (cli *DockerCli) Initialize(opts *cliflags.ClientOptions) error {
	cli.configFile = LoadDefaultConfigFile(cli.err)

	client, err := NewAPIClientFromFlags(opts.Common, cli.configFile)
	if err != nil {
		return err
	}

	cli.client = client

	if cli.in != nil {
		cli.inFd, cli.isTerminalIn = term.GetFdInfo(cli.in)
	}
	if cli.out != nil {
		cli.outFd, cli.isTerminalOut = term.GetFdInfo(cli.out)
	}

	if opts.Common.TrustKey == "" {
		cli.keyFile = filepath.Join(cliconfig.ConfigDir(), cliflags.DefaultTrustKeyFile)
	} else {
		cli.keyFile = opts.Common.TrustKey
	}

	return nil
}

// NewDockerCli returns a DockerCli instance with IO output and error streams set by in, out and err.
func NewDockerCli(in io.ReadCloser, out, err io.Writer) *DockerCli {
	return &DockerCli{
		in:  in,
		out: out,
		err: err,
	}
}

// LoadDefaultConfigFile attempts to load the default config file and returns
// an initialized ConfigFile struct if none is found.
func LoadDefaultConfigFile(err io.Writer) *configfile.ConfigFile {
	configFile, e := cliconfig.Load(cliconfig.ConfigDir())
	if e != nil {
		fmt.Fprintf(err, "WARNING: Error loading config file:%v\n", e)
	}
	if !configFile.ContainsAuth() {
		credentials.DetectDefaultStore(configFile)
	}
	return configFile
}

// NewAPIClientFromFlags creates a new APIClient from command line flags
func NewAPIClientFromFlags(opts *cliflags.CommonOptions, configFile *configfile.ConfigFile) (client.APIClient, error) {
	host, err := getServerHost(opts.Hosts, opts.TLSOptions)
	if err != nil {
		return &client.Client{}, err
	}

	customHeaders := configFile.HTTPHeaders
	if customHeaders == nil {
		customHeaders = map[string]string{}
	}
	customHeaders["User-Agent"] = clientUserAgent()

	verStr := api.DefaultVersion
	if tmpStr := os.Getenv("DOCKER_API_VERSION"); tmpStr != "" {
		verStr = tmpStr
	}

	httpClient, err := newHTTPClient(host, opts.TLSOptions)
	if err != nil {
		return &client.Client{}, err
	}

	return client.NewClient(host, verStr, httpClient, customHeaders)
}

func getServerHost(hosts []string, tlsOptions *tlsconfig.Options) (host string, err error) {
	switch len(hosts) {
	case 0:
		host = os.Getenv("DOCKER_HOST")
	case 1:
		host = hosts[0]
	default:
		return "", errors.New("Please specify only one -H")
	}

	host, err = dopts.ParseHost(tlsOptions != nil, host)
	return
}

func newHTTPClient(host string, tlsOptions *tlsconfig.Options) (*http.Client, error) {
	if tlsOptions == nil {
		// let the api client configure the default transport.
		return nil, nil
	}

	config, err := tlsconfig.Client(*tlsOptions)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		TLSClientConfig: config,
	}
	proto, addr, _, err := client.ParseHost(host)
	if err != nil {
		return nil, err
	}

	sockets.ConfigureTransport(tr, proto, addr)

	return &http.Client{
		Transport: tr,
	}, nil
}

func clientUserAgent() string {
	return "Docker-Client/" + dockerversion.Version + " (" + runtime.GOOS + ")"
}
