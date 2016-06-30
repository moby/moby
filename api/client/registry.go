package client

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	registrytypes "github.com/docker/engine-api/types/registry"
)

// ElectAuthServer returns the default registry to use (by asking the daemon)
func (cli *DockerCli) ElectAuthServer(ctx context.Context) string {
	// The daemon `/info` endpoint informs us of the default registry being
	// used. This is essential in cross-platforms environment, where for
	// example a Linux client might be interacting with a Windows daemon, hence
	// the default registry URL might be Windows specific.
	serverAddress := registry.IndexServer
	if info, err := cli.client.Info(ctx); err != nil {
		fmt.Fprintf(cli.out, "Warning: failed to get default registry endpoint from daemon (%v). Using system default: %s\n", err, serverAddress)
	} else {
		serverAddress = info.IndexServerAddress
	}
	return serverAddress
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func EncodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// RegistryAuthenticationPrivilegedFunc returns a RequestPrivilegeFunc from the specified registry index info
// for the given command.
func (cli *DockerCli) RegistryAuthenticationPrivilegedFunc(index *registrytypes.IndexInfo, cmdName string) types.RequestPrivilegeFunc {
	return func() (string, error) {
		fmt.Fprintf(cli.out, "\nPlease login prior to %s:\n", cmdName)
		indexServer := registry.GetAuthConfigKey(index)
		authConfig, err := cli.ConfigureAuth("", "", indexServer, false)
		if err != nil {
			return "", err
		}
		return EncodeAuthToBase64(authConfig)
	}
}

func (cli *DockerCli) promptWithDefault(prompt string, configDefault string) {
	if configDefault == "" {
		fmt.Fprintf(cli.out, "%s: ", prompt)
	} else {
		fmt.Fprintf(cli.out, "%s (%s): ", prompt, configDefault)
	}
}

// ResolveAuthConfig is like registry.ResolveAuthConfig, but if using the
// default index, it uses the default index name for the daemon's platform,
// not the client's platform.
func (cli *DockerCli) ResolveAuthConfig(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig {
	configKey := index.Name
	if index.Official {
		configKey = cli.ElectAuthServer(ctx)
	}

	a, _ := GetCredentials(cli.configFile, configKey)
	return a
}

// RetrieveAuthConfigs return all credentials.
func (cli *DockerCli) RetrieveAuthConfigs() map[string]types.AuthConfig {
	acs, _ := GetAllCredentials(cli.configFile)
	return acs
}

// ConfigureAuth returns an AuthConfig from the specified user, password and server.
func (cli *DockerCli) ConfigureAuth(flUser, flPassword, serverAddress string, isDefaultRegistry bool) (types.AuthConfig, error) {
	// On Windows, force the use of the regular OS stdin stream. Fixes #14336/#14210
	if runtime.GOOS == "windows" {
		cli.in = os.Stdin
	}

	authconfig, err := GetCredentials(cli.configFile, serverAddress)
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
		return authconfig, fmt.Errorf("Error: Cannot perform an interactive login from a non TTY device")
	}

	authconfig.Username = strings.TrimSpace(authconfig.Username)

	if flUser = strings.TrimSpace(flUser); flUser == "" {
		if isDefaultRegistry {
			// if this is a default registry (docker hub), then display the following message.
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

// resolveAuthConfigFromImage retrieves that AuthConfig using the image string
func (cli *DockerCli) resolveAuthConfigFromImage(ctx context.Context, image string) (types.AuthConfig, error) {
	registryRef, err := reference.ParseNamed(image)
	if err != nil {
		return types.AuthConfig{}, err
	}
	repoInfo, err := registry.ParseRepositoryInfo(registryRef)
	if err != nil {
		return types.AuthConfig{}, err
	}
	authConfig := cli.ResolveAuthConfig(ctx, repoInfo.Index)
	return authConfig, nil
}

// RetrieveAuthTokenFromImage retrieves an encoded auth token given a complete image
func (cli *DockerCli) RetrieveAuthTokenFromImage(ctx context.Context, image string) (string, error) {
	// Retrieve encoded auth token from the image reference
	authConfig, err := cli.resolveAuthConfigFromImage(ctx, image)
	if err != nil {
		return "", err
	}
	encodedAuth, err := EncodeAuthToBase64(authConfig)
	if err != nil {
		return "", err
	}
	return encodedAuth, nil
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
