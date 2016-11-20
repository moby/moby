package network

import (
	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewNetworkCommand returns a cobra command for `network` subcommands
func NewNetworkCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage networks",
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
	}
	cmd.AddCommand(
		newConnectCommand(dockerCli),
		newCreateCommand(dockerCli),
		newDisconnectCommand(dockerCli),
		newInspectCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
		NewPruneCommand(dockerCli),
	)
	return cmd
}
