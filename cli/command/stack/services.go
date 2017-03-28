package stack

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/cli/command/service"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

type servicesOptions struct {
	quiet     bool
	format    string
	filter    opts.FilterOpt
	namespace string
}

func newServicesCommand(dockerCli *command.DockerCli) *cobra.Command {
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
	flags.StringVar(&opts.format, "format", "", "Pretty-print services using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runServices(dockerCli *command.DockerCli, opts servicesOptions) error {
	ctx := context.Background()
	client := dockerCli.Client()

	filter := getStackFilterFromOpt(opts.namespace, opts.filter)
	services, err := client.ServiceList(ctx, types.ServiceListOptions{Filters: filter})
	if err != nil {
		return err
	}

	out := dockerCli.Out()

	// if no services in this stack, print message and exit 0
	if len(services) == 0 {
		fmt.Fprintf(out, "Nothing found in stack: %s\n", opts.namespace)
		return nil
	}

	info := map[string]formatter.ServiceListInfo{}
	if !opts.quiet {
		taskFilter := filters.NewArgs()
		for _, service := range services {
			taskFilter.Add("service", service.ID)
		}

		tasks, err := client.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})
		if err != nil {
			return err
		}

		nodes, err := client.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return err
		}

		info = service.GetServicesStatus(services, nodes, tasks)
	}

	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().ServicesFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().ServicesFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	servicesCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewServiceListFormat(format, opts.quiet),
	}
	return formatter.ServiceListWrite(servicesCtx, services, info)
}
