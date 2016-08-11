package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

func newDrainCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "drain NODE [NODE...]",
		Short: "Drain a node in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDrain(dockerCli, args)
		},
	}
}

func runDrain(dockerCli *client.DockerCli, nodes []string) error {
	drain := func(node *swarm.Node) error {
		node.Spec.Availability = swarm.NodeAvailabilityDrain
		return nil
	}
	success := func(nodeID string) {
		fmt.Fprintf(dockerCli.Out(), "Node %s availability set to drain.\n", nodeID)
	}
	return updateNodes(dockerCli, nodes, drain, success)
}
