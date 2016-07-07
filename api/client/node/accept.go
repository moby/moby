package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

func newAcceptCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "accept NODE [NODE...]",
		Short: "Accept a node in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccept(dockerCli, args)
		},
	}
}

func runAccept(dockerCli *client.DockerCli, nodes []string) error {
	accept := func(node *swarm.Node) {
		node.Spec.Membership = swarm.NodeMembershipAccepted
	}
	success := func(nodeID string) {
		fmt.Fprintf(dockerCli.Out(), "Node %s accepted in the swarm.\n", nodeID)
	}
	return updateNodes(dockerCli, nodes, accept, success)
}
