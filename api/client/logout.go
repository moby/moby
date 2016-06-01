package client

import (
	"fmt"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdLogout logs a user out from a Docker registry.
//
// If no server is specified, the user will be logged out from the registry's index server.
//
// Usage: docker logout [SERVER]
func (cli *DockerCli) CmdLogout(args ...string) error {
	cmd := Cli.Subcmd("logout", []string{"[SERVER]"}, Cli.DockerCommands["logout"].Description+".\nIf no server is specified, the default is defined by the daemon.", true)
	cmd.Require(flag.Max, 1)

	cmd.ParseFlags(args, true)

	var serverAddress string
	if len(cmd.Args()) > 0 {
		serverAddress = cmd.Arg(0)
	} else {
		serverAddress = cli.electAuthServer(context.Background())
	}

	// check if we're logged in based on the records in the config file
	// which means it couldn't have user/pass cause they may be in the creds store
	if _, ok := cli.configFile.AuthConfigs[serverAddress]; !ok {
		fmt.Fprintf(cli.out, "Not logged in to %s\n", serverAddress)
		return nil
	}

	fmt.Fprintf(cli.out, "Remove login credentials for %s\n", serverAddress)
	if err := eraseCredentials(cli.configFile, serverAddress); err != nil {
		fmt.Fprintf(cli.out, "WARNING: could not erase credentials: %v\n", err)
	}

	return nil
}
