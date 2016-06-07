package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts nodeOptions
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] NODE",
		Short: "Update a node",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, args[0], mergeNodeUpdate(flags))
		},
	}

	flags = cmd.Flags()
	flags.StringVar(&opts.role, "role", "", "Role of the node (worker/manager)")
	flags.StringVar(&opts.membership, "membership", "", "Membership of the node (accepted/rejected)")
	flags.StringVar(&opts.availability, "availability", "", "Availability of the node (active/pause/drain)")
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, nodeID string, mergeNode func(node *swarm.Node)) error {
	client := dockerCli.Client()
	ctx := context.Background()

	node, err := client.NodeInspect(ctx, nodeID)
	if err != nil {
		return err
	}

	mergeNode(&node)
	err = client.NodeUpdate(ctx, nodeID, node)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", nodeID)
	return nil
}

func mergeNodeUpdate(flags *pflag.FlagSet) func(*swarm.Node) {
	return func(node *swarm.Node) {
		mergeString := func(flag string, field *string) {
			if flags.Changed(flag) {
				*field, _ = flags.GetString(flag)
			}
		}

		mergeRole := func(flag string, field *swarm.NodeRole) {
			if flags.Changed(flag) {
				str, _ := flags.GetString(flag)
				*field = swarm.NodeRole(str)
			}
		}

		mergeMembership := func(flag string, field *swarm.NodeMembership) {
			if flags.Changed(flag) {
				str, _ := flags.GetString(flag)
				*field = swarm.NodeMembership(str)
			}
		}

		mergeAvailability := func(flag string, field *swarm.NodeAvailability) {
			if flags.Changed(flag) {
				str, _ := flags.GetString(flag)
				*field = swarm.NodeAvailability(str)
			}
		}

		mergeLabels := func(flag string, field *map[string]string) {
			if flags.Changed(flag) {
				values, _ := flags.GetStringSlice(flag)
				for key, value := range runconfigopts.ConvertKVStringsToMap(values) {
					(*field)[key] = value
				}
			}
		}

		spec := &node.Spec
		mergeString("name", &spec.Name)
		// TODO: setting labels is not working
		mergeLabels("label", &spec.Labels)
		mergeRole("role", &spec.Role)
		mergeMembership("membership", &spec.Membership)
		mergeAvailability("availability", &spec.Availability)
	}
}
