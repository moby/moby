package node

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/idresolver"
	"github.com/docker/docker/api/client/task"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type psOptions struct {
	nodeID    string
	noResolve bool
	noTrunc   bool
	filter    opts.FilterOpt
}

func newPsCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] [NODE]",
		Short: "List tasks running on a node, defaults to current node",
		Args:  cli.RequiresRangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.nodeID = "self"

			if len(args) != 0 {
				opts.nodeID = args[0]
			}

			return runPs(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "Do not map IDs to Names")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runPs(dockerCli *client.DockerCli, opts psOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	nodeRef, err := Reference(client, ctx, opts.nodeID)
	if err != nil {
		return nil
	}
	node, _, err := client.NodeInspectWithRaw(ctx, nodeRef)
	if err != nil {
		return err
	}

	filter := opts.filter.Value()
	filter.Add("node", node.ID)
	tasks, err := client.TaskList(
		ctx,
		types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve), opts.noTrunc)
}
