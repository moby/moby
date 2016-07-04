package registry

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

// NewLogoutCommand creates a new `docker login` command
func NewLogoutCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [SERVER]",
		Short: "Log out from a Docker registry.",
		Long:  "Log out from a Docker registry.\nIf no server is specified, the default is defined by the daemon.",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var serverAddress string
			if len(args) > 0 {
				serverAddress = args[0]
			}
			return runLogout(dockerCli, serverAddress)
		},
	}

	return cmd
}

func runLogout(dockerCli *client.DockerCli, serverAddress string) error {
	ctx := context.Background()

	if serverAddress == "" {
		serverAddress = dockerCli.ElectAuthServer(ctx)
	}

	// check if we're logged in based on the records in the config file
	// which means it couldn't have user/pass cause they may be in the creds store
	if _, ok := dockerCli.ConfigFile().AuthConfigs[serverAddress]; !ok {
		fmt.Fprintf(dockerCli.Out(), "Not logged in to %s\n", serverAddress)
		return nil
	}

	fmt.Fprintf(dockerCli.Out(), "Remove login credentials for %s\n", serverAddress)
	if err := client.EraseCredentials(dockerCli.ConfigFile(), serverAddress); err != nil {
		fmt.Fprintf(dockerCli.Err(), "WARNING: could not erase credentials: %v\n", err)
	}

	return nil
}
