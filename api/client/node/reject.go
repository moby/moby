package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newRejectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "reject NODE [NODE...]",
		Short: "Reject a node from the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReject(dockerCli, flags, args)
		},
	}

	flags = cmd.Flags()
	return cmd
}

func runReject(dockerCli *client.DockerCli, flags *pflag.FlagSet, args []string) error {
	for _, id := range args {
		if err := runUpdate(dockerCli, id, func(node *swarm.Node) {
			node.Spec.Membership = swarm.NodeMembershipRejected
		}); err != nil {
			return err
		}
		fmt.Println(id, "attempting to reject a node from the swarm.")
	}

	return nil
}
