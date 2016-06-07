package service

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
	serviceID string
	all       bool
	noResolve bool
	filter    opts.FilterOpt
}

func newTasksCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := tasksOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "tasks [OPTIONS] SERVICE",
		Short: "List the tasks of a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.serviceID = args[0]
			return runTasks(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&opts.all, "all", "a", false, "Display all tasks")
	flags.BoolVarP(&opts.noResolve, "no-resolve", "n", false, "Do not map IDs to Names")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runTasks(dockerCli *client.DockerCli, opts tasksOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	service, err := client.ServiceInspect(ctx, opts.serviceID)
	if err != nil {
		return err
	}

	filter := opts.filter.Value()
	filter.Add("service", service.ID)
	if !opts.all && !filter.Include("desired_state") {
		filter.Add("desired_state", swarm.TaskStateRunning)
		filter.Add("desired_state", swarm.TaskStateAccepted)
	}

	tasks, err := client.TaskList(ctx, types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve))
}
