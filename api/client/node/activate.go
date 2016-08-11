package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

func newActivateCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "activate NODE [NODE...]",
		Short: "Activate a node in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActivate(dockerCli, args)
		},
	}
}

func runActivate(dockerCli *client.DockerCli, nodes []string) error {
	activate := func(node *swarm.Node) error {
		node.Spec.Availability = swarm.NodeAvailabilityActive
		return nil
	}
	success := func(nodeID string) {
		fmt.Fprintf(dockerCli.Out(), "Node %s availability set to active.\n", nodeID)
	}
	return updateNodes(dockerCli, nodes, activate, success)
}
