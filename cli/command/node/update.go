package node

import (
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

var (
	errNoRoleChange = errors.New("role was already set to the requested value")
)

func newUpdateCommand(dockerCli command.Cli) *cobra.Command {
	nodeOpts := newNodeOptions()

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] NODE",
		Short: "Update a node",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), args[0])
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&nodeOpts.role, flagRole, "", "Role of the node (worker/manager)")
	flags.StringVar(&nodeOpts.availability, flagAvailability, "", "Availability of the node (active/pause/drain)")
	flags.Var(&nodeOpts.annotations.labels, flagLabelAdd, "Add or update a node label (key=value)")
	labelKeys := opts.NewListOpts(nil)
	flags.Var(&labelKeys, flagLabelRemove, "Remove a node label if exists")
	return cmd
}

func runUpdate(dockerCli command.Cli, flags *pflag.FlagSet, nodeID string) error {
	success := func(_ string) {
		fmt.Fprintln(dockerCli.Out(), nodeID)
	}
	return updateNodes(dockerCli, []string{nodeID}, mergeNodeUpdate(flags), success)
}

func updateNodes(dockerCli command.Cli, nodes []string, mergeNode func(node *swarm.Node) error, success func(nodeID string)) error {
	client := dockerCli.Client()
	ctx := context.Background()

	for _, nodeID := range nodes {
		node, _, err := client.NodeInspectWithRaw(ctx, nodeID)
		if err != nil {
			return err
		}

		err = mergeNode(&node)
		if err != nil {
			if err == errNoRoleChange {
				continue
			}
			return err
		}
		err = client.NodeUpdate(ctx, node.ID, node.Version, node.Spec)
		if err != nil {
			return err
		}
		success(nodeID)
	}
	return nil
}

func mergeNodeUpdate(flags *pflag.FlagSet) func(*swarm.Node) error {
	return func(node *swarm.Node) error {
		spec := &node.Spec

		if flags.Changed(flagRole) {
			str, err := flags.GetString(flagRole)
			if err != nil {
				return err
			}
			spec.Role = swarm.NodeRole(str)
		}
		if flags.Changed(flagAvailability) {
			str, err := flags.GetString(flagAvailability)
			if err != nil {
				return err
			}
			spec.Availability = swarm.NodeAvailability(str)
		}
		if spec.Annotations.Labels == nil {
			spec.Annotations.Labels = make(map[string]string)
		}
		if flags.Changed(flagLabelAdd) {
			labels := flags.Lookup(flagLabelAdd).Value.(*opts.ListOpts).GetAll()
			for k, v := range runconfigopts.ConvertKVStringsToMap(labels) {
				spec.Annotations.Labels[k] = v
			}
		}
		if flags.Changed(flagLabelRemove) {
			keys := flags.Lookup(flagLabelRemove).Value.(*opts.ListOpts).GetAll()
			for _, k := range keys {
				// if a key doesn't exist, fail the command explicitly
				if _, exists := spec.Annotations.Labels[k]; !exists {
					return fmt.Errorf("key %s doesn't exist in node's labels", k)
				}
				delete(spec.Annotations.Labels, k)
			}
		}
		return nil
	}
}

const (
	flagRole         = "role"
	flagAvailability = "availability"
	flagLabelAdd     = "label-add"
	flagLabelRemove  = "label-rm"
)
