package node

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/idresolver"
	"github.com/docker/docker/api/client/task"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type psOptions struct {
	nodeID    string
	all       bool
	self      bool
	noResolve bool
	filter    opts.FilterOpt
}

func newPSCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] [NODE]",
		Short: "List tasks running on a node (default to current node)",
		Args:  nil,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"self"}
			}

			opts.nodeID = args[0]

			return runPS(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&opts.all, "all", "a", false, "Show all tasks (default shows tasks that are or will be running)")
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "Do not map IDs to Names")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runPS(dockerCli *client.DockerCli, opts psOptions) error {
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

	options := types.TaskListOptions{
		Filter: filter,
		All:    opts.all,
	}

	tasks, err := client.TaskList(ctx, options)
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve))
}
