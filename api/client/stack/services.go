// +build experimental

package stack

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/service"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/spf13/cobra"
)

const (
	listItemFmt = "%s\t%s\t%s\t%s\t%s\n"
)

type servicesOptions struct {
	quiet     bool
	filter    opts.FilterOpt
	namespace string
}

func newServicesCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := servicesOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "services [OPTIONS] STACK",
		Short: "List the services in the stack",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.namespace = args[0]
			return runServices(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display IDs")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runServices(dockerCli *client.DockerCli, opts servicesOptions) error {
	ctx := context.Background()
	client := dockerCli.Client()

	filter := opts.filter.Value()
	filter.Add("label", labelNamespace+"="+opts.namespace)

	services, err := client.ServiceList(ctx, types.ServiceListOptions{Filter: filter})
	if err != nil {
		return err
	}

	out := dockerCli.Out()

	// if no services in this stack, print message and exit 0
	if len(services) == 0 {
		fmt.Fprintf(out, "Nothing found in stack: %s\n", opts.namespace)
		return nil
	}

	if opts.quiet {
		service.PrintQuiet(out, services)
	} else {
		taskFilter := filters.NewArgs()
		for _, service := range services {
			taskFilter.Add("service", service.ID)
		}

		tasks, err := client.TaskList(ctx, types.TaskListOptions{Filter: taskFilter})
		if err != nil {
			return err
		}
		nodes, err := client.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return err
		}
		service.PrintNotQuiet(out, services, nodes, tasks)
	}
	return nil
}
