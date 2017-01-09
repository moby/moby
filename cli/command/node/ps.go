package node

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/docker/cli/command/task"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type psOptions struct {
	nodeIDs   []string
	noResolve bool
	noTrunc   bool
	filter    opts.FilterOpt
}

func newPsCommand(dockerCli command.Cli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] [NODE...]",
		Short: "List tasks running on one or more nodes, defaults to current node",
		Args:  cli.RequiresMinArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.nodeIDs = []string{"self"}

			if len(args) != 0 {
				opts.nodeIDs = args
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

func runPs(dockerCli command.Cli, opts psOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	var (
		errs  []string
		tasks []swarm.Task
	)

	for _, nodeID := range opts.nodeIDs {
		nodeRef, err := Reference(ctx, client, nodeID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		node, _, err := client.NodeInspectWithRaw(ctx, nodeRef)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		filter := opts.filter.Value()
		filter.Add("node", node.ID)

		nodeTasks, err := client.TaskList(ctx, types.TaskListOptions{Filters: filter})
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		tasks = append(tasks, nodeTasks...)
	}

	if err := task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve), opts.noTrunc); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
