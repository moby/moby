package command

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/docker/cliconfig/credentials"
	"github.com/docker/docker/client"
	"github.com/docker/docker/dockerversion"
	dopts "github.com/docker/docker/opts"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// Streams is an interface which exposes the standard input and output streams
type Streams interface {
	In() *InStream
	Out() *OutStream
	Err() io.Writer
}

// DockerCli represents the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	configFile      *configfile.ConfigFile
	in              *InStream
	out             *OutStream
	err             io.Writer
	keyFile         string
	client          client.APIClient
	hasExperimental bool
	defaultVersion  string
}

// HasExperimental returns true if experimental features are accessible.
func (cli *DockerCli) HasExperimental() bool {
	return cli.hasExperimental
}

// DefaultVersion returns api.defaultVersion of DOCKER_API_VERSION if specified.
func (cli *DockerCli) DefaultVersion() string {
	return cli.defaultVersion
}

// Client returns the APIClient
func (cli *DockerCli) Client() client.APIClient {
	return cli.client
}

// Out returns the writer used for stdout
func (cli *DockerCli) Out() *OutStream {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *DockerCli) Err() io.Writer {
	return cli.err
}

// In returns the reader used for stdin
func (cli *DockerCli) In() *InStream {
	return cli.in
}

// ShowHelp shows the command help.
func (cli *DockerCli) ShowHelp(cmd *cobra.Command, args []string) error {
	cmd.SetOutput(cli.err)
	cmd.HelpFunc()(cmd, args)
	return nil
}

// ConfigFile returns the ConfigFile
func (cli *DockerCli) ConfigFile() *configfile.ConfigFile {
	return cli.configFile
}

// GetAllCredentials returns all of the credentials stored in all of the
// configured credential stores.
func (cli *DockerCli) GetAllCredentials() (map[string]types.AuthConfig, error) {
	auths := make(map[string]types.AuthConfig)
	for registry := range cli.configFile.CredentialHelpers {
		helper := cli.CredentialsStore(registry)
		newAuths, err := helper.GetAll()
		if err != nil {
			return nil, err
		}
		addAll(auths, newAuths)
	}
	defaultStore := cli.CredentialsStore("")
	newAuths, err := defaultStore.GetAll()
	if err != nil {
		return nil, err
	}
	addAll(auths, newAuths)
	return auths, nil
}

func addAll(to, from map[string]types.AuthConfig) {
	for reg, ac := range from {
		to[reg] = ac
	}
}

// CredentialsStore returns a new credentials store based
// on the settings provided in the configuration file. Empty string returns
// the default credential store.
func (cli *DockerCli) CredentialsStore(serverAddress string) credentials.Store {
	if helper := getConfiguredCredentialStore(cli.configFile, serverAddress); helper != "" {
		return credentials.NewNativeStore(cli.configFile, helper)
	}
	return credentials.NewFileStore(cli.configFile)
}

// getConfiguredCredentialStore returns the credential helper configured for the
// given registry, the default credsStore, or the empty string if neither are
// configured.
func getConfiguredCredentialStore(c *configfile.ConfigFile, serverAddress string) string {
	if c.CredentialHelpers != nil && serverAddress != "" {
		if helper, exists := c.CredentialHelpers[serverAddress]; exists {
			return helper
		}
	}
	return c.CredentialsStore
}

// Initialize the dockerCli runs initialization that must happen after command
// line flags are parsed.
func (cli *DockerCli) Initialize(opts *cliflags.ClientOptions) error {
	cli.configFile = LoadDefaultConfigFile(cli.err)

	var err error
	cli.client, err = NewAPIClientFromFlags(opts.Common, cli.configFile)
	if err != nil {
		return err
	}

	cli.defaultVersion = cli.client.ClientVersion()

	if opts.Common.TrustKey == "" {
		cli.keyFile = filepath.Join(cliconfig.ConfigDir(), cliflags.DefaultTrustKeyFile)
	} else {
		cli.keyFile = opts.Common.TrustKey
	}

	if ping, err := cli.client.Ping(context.Background()); err == nil {
		cli.hasExperimental = ping.Experimental

		// since the new header was added in 1.25, assume server is 1.24 if header is not present.
		if ping.APIVersion == "" {
			ping.APIVersion = "1.24"
		}

		// if server version is lower than the current cli, downgrade
		if versions.LessThan(ping.APIVersion, cli.client.ClientVersion()) {
			cli.client.UpdateClientVersion(ping.APIVersion)
		}
	}
	return nil
}

// NewDockerCli returns a DockerCli instance with IO output and error streams set by in, out and err.
func NewDockerCli(in io.ReadCloser, out, err io.Writer) *DockerCli {
	return &DockerCli{in: NewInStream(in), out: NewOutStream(out), err: err}
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
	customHeaders["User-Agent"] = UserAgent()

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

// UserAgent returns the user agent string used for making API requests
func UserAgent() string {
	return "Docker-Client/" + dockerversion.Version + " (" + runtime.GOOS + ")"
}
