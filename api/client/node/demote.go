package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newDemoteCommand(dockerCli *client.DockerCli) *cobra.Command {
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "demote NODE [NODE...]",
		Short: "Demote a node as manager in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDemote(dockerCli, flags, args)
		},
	}

	flags = cmd.Flags()
	return cmd
}

func runDemote(dockerCli *client.DockerCli, flags *pflag.FlagSet, args []string) error {
	for _, id := range args {
		if err := runUpdate(dockerCli, id, func(node *swarm.Node) {
			node.Spec.Role = swarm.NodeRoleWorker
		}); err != nil {
			return err
		}
		fmt.Println(id, "attempting to demote a manager in the swarm.")
	}

	return nil
}
