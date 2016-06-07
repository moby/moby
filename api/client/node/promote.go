package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newPromoteCommand(dockerCli *client.DockerCli) *cobra.Command {
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "promote NODE [NODE...]",
		Short: "Promote a node as manager in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPromote(dockerCli, flags, args)
		},
	}

	flags = cmd.Flags()
	return cmd
}

func runPromote(dockerCli *client.DockerCli, flags *pflag.FlagSet, args []string) error {
	for _, id := range args {
		if err := runUpdate(dockerCli, id, func(node *swarm.Node) {
			node.Spec.Role = swarm.NodeRoleManager
		}); err != nil {
			return err
		}
		fmt.Println(id, "attempting to promote a node to a manager in the swarm.")
	}

	return nil
}
