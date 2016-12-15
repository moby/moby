package registry

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
)

// NewLogoutCommand creates a new `docker login` command
func NewLogoutCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [SERVER]",
		Short: "Log out from a Docker registry",
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

func runLogout(dockerCli *command.DockerCli, serverAddress string) error {
	ctx := context.Background()
	var isDefaultRegistry bool

	if serverAddress == "" {
		serverAddress = command.ElectAuthServer(ctx, dockerCli)
		isDefaultRegistry = true
	}

	var (
		loggedIn        bool
		regsToLogout    []string
		hostnameAddress = serverAddress
		regsToTry       = []string{serverAddress}
	)
	if !isDefaultRegistry {
		hostnameAddress = registry.ConvertToHostname(serverAddress)
		// the tries below are kept for backward compatibility where a user could have
		// saved the registry in one of the following format.
		regsToTry = append(regsToTry, hostnameAddress, "http://"+hostnameAddress, "https://"+hostnameAddress)
	}

	// check if we're logged in based on the records in the config file
	// which means it couldn't have user/pass cause they may be in the creds store
	for _, s := range regsToTry {
		if _, ok := dockerCli.ConfigFile().AuthConfigs[s]; ok {
			loggedIn = true
			regsToLogout = append(regsToLogout, s)
		}
	}

	if !loggedIn {
		fmt.Fprintf(dockerCli.Out(), "Not logged in to %s\n", hostnameAddress)
		return nil
	}

	fmt.Fprintf(dockerCli.Out(), "Removing login credentials for %s\n", hostnameAddress)
	for _, r := range regsToLogout {
		if err := dockerCli.CredentialsStore(r).Erase(r); err != nil {
			fmt.Fprintf(dockerCli.Err(), "WARNING: could not erase credentials: %v\n", err)
		}
	}

	return nil
}
