// +build experimental

package stack

import (
	"fmt"

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
	all       bool
	filter    opts.FilterOpt
	namespace string
	noResolve bool
}

func newTasksCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := tasksOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "tasks [OPTIONS] STACK",
		Short: "List the tasks in the stack",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.namespace = args[0]
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
	namespace := opts.namespace
	client := dockerCli.Client()
	ctx := context.Background()

	filter := opts.filter.Value()
	filter.Add("label", labelNamespace+"="+opts.namespace)
	if !opts.all && !filter.Include("desired-state") {
		filter.Add("desired-state", string(swarm.TaskStateRunning))
		filter.Add("desired-state", string(swarm.TaskStateAccepted))
	}

	tasks, err := client.TaskList(ctx, types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Fprintf(dockerCli.Out(), "Nothing found in stack: %s\n", namespace)
		return nil
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve))
}
