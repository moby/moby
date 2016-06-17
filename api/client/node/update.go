package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts nodeOptions

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] NODE",
		Short: "Update a node",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), args[0])
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.role, flagRole, "", "Role of the node (worker/manager)")
	flags.StringVar(&opts.membership, flagMembership, "", "Membership of the node (accepted/rejected)")
	flags.StringVar(&opts.availability, flagAvailability, "", "Availability of the node (active/pause/drain)")
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, flags *pflag.FlagSet, nodeID string) error {
	success := func(_ string) {
		fmt.Fprintln(dockerCli.Out(), nodeID)
	}
	return updateNodes(dockerCli, []string{nodeID}, mergeNodeUpdate(flags), success)
}

func updateNodes(dockerCli *client.DockerCli, nodes []string, mergeNode func(node *swarm.Node), success func(nodeID string)) error {
	client := dockerCli.Client()
	ctx := context.Background()

	for _, nodeID := range nodes {
		node, err := client.NodeInspect(ctx, nodeID)
		if err != nil {
			return err
		}

		mergeNode(&node)
		err = client.NodeUpdate(ctx, node.ID, node.Version, node.Spec)
		if err != nil {
			return err
		}
		success(nodeID)
	}
	return nil
}

func mergeNodeUpdate(flags *pflag.FlagSet) func(*swarm.Node) {
	return func(node *swarm.Node) {
		spec := &node.Spec

		if flags.Changed(flagRole) {
			str, _ := flags.GetString(flagRole)
			spec.Role = swarm.NodeRole(str)
		}
		if flags.Changed(flagMembership) {
			str, _ := flags.GetString(flagMembership)
			spec.Membership = swarm.NodeMembership(str)
		}
		if flags.Changed(flagAvailability) {
			str, _ := flags.GetString(flagAvailability)
			spec.Availability = swarm.NodeAvailability(str)
		}
	}
}

const (
	flagRole         = "role"
	flagMembership   = "membership"
	flagAvailability = "availability"
)
