package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

func newPauseCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "pause NODE [NODE...]",
		Short: "Pause a node in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPause(dockerCli, args)
		},
	}
}

func runPause(dockerCli *client.DockerCli, nodes []string) error {
	pause := func(node *swarm.Node) error {
		node.Spec.Availability = swarm.NodeAvailabilityPause
		return nil
	}
	success := func(nodeID string) {
		fmt.Fprintf(dockerCli.Out(), "Node %s availability set to pause.\n", nodeID)
	}
	return updateNodes(dockerCli, nodes, pause, success)
}
