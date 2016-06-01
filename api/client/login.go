package client

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/docker/cliconfig/credentials"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/types"
)

// CmdLogin logs in a user to a Docker registry service.
//
// If no server is specified, the user will be logged into or registered to the registry's index server.
//
// Usage: docker login SERVER
func (cli *DockerCli) CmdLogin(args ...string) error {
	cmd := Cli.Subcmd("login", []string{"[SERVER]"}, Cli.DockerCommands["login"].Description+".\nIf no server is specified, the default is defined by the daemon.", true)
	cmd.Require(flag.Max, 1)

	flUser := cmd.String([]string{"u", "-username"}, "", "Username")
	flPassword := cmd.String([]string{"p", "-password"}, "", "Password")

	// Deprecated in 1.11: Should be removed in docker 1.13
	cmd.String([]string{"#e", "#-email"}, "", "Email")

	cmd.ParseFlags(args, true)

	// On Windows, force the use of the regular OS stdin stream. Fixes #14336/#14210
	if runtime.GOOS == "windows" {
		cli.in = os.Stdin
	}

	ctx := context.Background()
	var serverAddress string
	var isDefaultRegistry bool
	if len(cmd.Args()) > 0 {
		serverAddress = cmd.Arg(0)
	} else {
		serverAddress = cli.electAuthServer(ctx)
		isDefaultRegistry = true
	}
	authConfig, err := cli.configureAuth(*flUser, *flPassword, serverAddress, isDefaultRegistry)
	if err != nil {
		return err
	}
	response, err := cli.client.RegistryLogin(ctx, authConfig)
	if err != nil {
		return err
	}
	if response.IdentityToken != "" {
		authConfig.Password = ""
		authConfig.IdentityToken = response.IdentityToken
	}
	if err := storeCredentials(cli.configFile, authConfig); err != nil {
		return fmt.Errorf("Error saving credentials: %v", err)
	}

	if response.Status != "" {
		fmt.Fprintln(cli.out, response.Status)
	}
	return nil
}

func (cli *DockerCli) promptWithDefault(prompt string, configDefault string) {
	if configDefault == "" {
		fmt.Fprintf(cli.out, "%s: ", prompt)
	} else {
		fmt.Fprintf(cli.out, "%s (%s): ", prompt, configDefault)
	}
}

func (cli *DockerCli) configureAuth(flUser, flPassword, serverAddress string, isDefaultRegistry bool) (types.AuthConfig, error) {
	authconfig, err := getCredentials(cli.configFile, serverAddress)
	if err != nil {
		return authconfig, err
	}

	// Some links documenting this:
	// - https://code.google.com/archive/p/mintty/issues/56
	// - https://github.com/docker/docker/issues/15272
	// - https://mintty.github.io/ (compatibility)
	// Linux will hit this if you attempt `cat | docker login`, and Windows
	// will hit this if you attempt docker login from mintty where stdin
	// is a pipe, not a character based console.
	if flPassword == "" && !cli.isTerminalIn {
		return authconfig, fmt.Errorf("Error: Cannot perform an interactive logon from a non TTY device")
	}

	authconfig.Username = strings.TrimSpace(authconfig.Username)

	if flUser = strings.TrimSpace(flUser); flUser == "" {
		if isDefaultRegistry {
			// if this is a defauly registry (docker hub), then display the following message.
			fmt.Fprintln(cli.out, "Login with your Docker ID to push and pull images from Docker Hub. If you don't have a Docker ID, head over to https://hub.docker.com to create one.")
		}
		cli.promptWithDefault("Username", authconfig.Username)
		flUser = readInput(cli.in, cli.out)
		flUser = strings.TrimSpace(flUser)
		if flUser == "" {
			flUser = authconfig.Username
		}
	}
	if flUser == "" {
		return authconfig, fmt.Errorf("Error: Non-null Username Required")
	}
	if flPassword == "" {
		oldState, err := term.SaveState(cli.inFd)
		if err != nil {
			return authconfig, err
		}
		fmt.Fprintf(cli.out, "Password: ")
		term.DisableEcho(cli.inFd, oldState)

		flPassword = readInput(cli.in, cli.out)
		fmt.Fprint(cli.out, "\n")

		term.RestoreTerminal(cli.inFd, oldState)
		if flPassword == "" {
			return authconfig, fmt.Errorf("Error: Password Required")
		}
	}

	authconfig.Username = flUser
	authconfig.Password = flPassword
	authconfig.ServerAddress = serverAddress
	authconfig.IdentityToken = ""

	return authconfig, nil
}

func readInput(in io.Reader, out io.Writer) string {
	reader := bufio.NewReader(in)
	line, _, err := reader.ReadLine()
	if err != nil {
		fmt.Fprintln(out, err.Error())
		os.Exit(1)
	}
	return string(line)
}

// getCredentials loads the user credentials from a credentials store.
// The store is determined by the config file settings.
func getCredentials(c *configfile.ConfigFile, serverAddress string) (types.AuthConfig, error) {
	s := loadCredentialsStore(c)
	return s.Get(serverAddress)
}

func getAllCredentials(c *configfile.ConfigFile) (map[string]types.AuthConfig, error) {
	s := loadCredentialsStore(c)
	return s.GetAll()
}

// storeCredentials saves the user credentials in a credentials store.
// The store is determined by the config file settings.
func storeCredentials(c *configfile.ConfigFile, auth types.AuthConfig) error {
	s := loadCredentialsStore(c)
	return s.Store(auth)
}

// eraseCredentials removes the user credentials from a credentials store.
// The store is determined by the config file settings.
func eraseCredentials(c *configfile.ConfigFile, serverAddress string) error {
	s := loadCredentialsStore(c)
	return s.Erase(serverAddress)
}

// loadCredentialsStore initializes a new credentials store based
// in the settings provided in the configuration file.
func loadCredentialsStore(c *configfile.ConfigFile) credentials.Store {
	if c.CredentialsStore != "" {
		return credentials.NewNativeStore(c)
	}
	return credentials.NewFileStore(c)
}
