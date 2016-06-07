package node

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/idresolver"
	"github.com/docker/docker/api/client/task"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

type tasksOptions struct {
	nodeID    string
	all       bool
	noResolve bool
	filter    opts.FilterOpt
}

func newTasksCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := tasksOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "tasks [OPTIONS] self|NODE",
		Short: "List tasks running on a node",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.nodeID = args[0]
			return runTasks(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&opts.all, "all", "a", false, "Display all instances")
	flags.BoolVarP(&opts.noResolve, "no-resolve", "n", false, "Do not map IDs to Names")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runTasks(dockerCli *client.DockerCli, opts tasksOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	nodeRef, err := nodeReference(client, ctx, opts.nodeID)
	if err != nil {
		return nil
	}
	node, err := client.NodeInspect(ctx, nodeRef)
	if err != nil {
		return err
	}

	filter := opts.filter.Value()
	filter.Add("node", node.ID)
	if !opts.all {
		filter.Add("desired_state", swarm.TaskStateRunning)
		filter.Add("desired_state", swarm.TaskStateAccepted)

	}

	tasks, err := client.TaskList(
		ctx,
		types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve))
}
