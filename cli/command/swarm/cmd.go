package swarm

import (
	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewSwarmCommand returns a cobra command for `swarm` subcommands
func NewSwarmCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "Manage Swarm",
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
		Tags:  map[string]string{"version": "1.24"},
	}
	cmd.AddCommand(
		newInitCommand(dockerCli),
		newJoinCommand(dockerCli),
		newJoinTokenCommand(dockerCli),
		newUnlockKeyCommand(dockerCli),
		newUpdateCommand(dockerCli),
		newLeaveCommand(dockerCli),
		newUnlockCommand(dockerCli),
	)
	return cmd
}
