package service

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/docker/cli/command/node"
	"github.com/docker/docker/cli/command/task"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type psOptions struct {
	serviceID string
	noResolve bool
	noTrunc   bool
	filter    opts.FilterOpt
}

func newPsCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] SERVICE",
		Short: "List the tasks of a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.serviceID = args[0]
			return runPS(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "Do not map IDs to Names")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runPS(dockerCli *command.DockerCli, opts psOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	service, _, err := client.ServiceInspectWithRaw(ctx, opts.serviceID)
	if err != nil {
		return err
	}

	filter := opts.filter.Value()
	filter.Add("service", service.ID)
	if filter.Include("node") {
		nodeFilters := filter.Get("node")
		for _, nodeFilter := range nodeFilters {
			nodeReference, err := node.Reference(ctx, client, nodeFilter)
			if err != nil {
				return err
			}
			filter.Del("node", nodeFilter)
			filter.Add("node", nodeReference)
		}
	}

	tasks, err := client.TaskList(ctx, types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve), opts.noTrunc)
}
