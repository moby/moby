package swarm

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
)

// NewSwarmCommand returns a cobra command for `swarm` subcommands
func NewSwarmCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "Manage docker swarm",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
		},
	}
	cmd.AddCommand(
		newInitCommand(dockerCli),
		newJoinCommand(dockerCli),
		newUpdateCommand(dockerCli),
		newLeaveCommand(dockerCli),
		newInspectCommand(dockerCli),
	)
	return cmd
}
