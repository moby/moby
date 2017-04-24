package service

import (
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/docker/cli/command/node"
	"github.com/docker/docker/cli/command/task"
	"github.com/docker/docker/opts"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type psOptions struct {
	services  []string
	quiet     bool
	noResolve bool
	noTrunc   bool
	format    string
	filter    opts.FilterOpt
}

func newPsCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] SERVICE [SERVICE...]",
		Short: "List the tasks of one or more services",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.services = args
			return runPS(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display task IDs")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "Do not map IDs to Names")
	flags.StringVar(&opts.format, "format", "", "Pretty-print tasks using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runPS(dockerCli *command.DockerCli, opts psOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	filter := opts.filter.Value()

	serviceIDFilter := filters.NewArgs()
	serviceNameFilter := filters.NewArgs()
	for _, service := range opts.services {
		serviceIDFilter.Add("id", service)
		serviceNameFilter.Add("name", service)
	}
	serviceByIDList, err := client.ServiceList(ctx, types.ServiceListOptions{Filters: serviceIDFilter})
	if err != nil {
		return err
	}
	serviceByNameList, err := client.ServiceList(ctx, types.ServiceListOptions{Filters: serviceNameFilter})
	if err != nil {
		return err
	}

	var errs []string
	serviceCount := 0
loop:
	// Match services by 1. Full ID, 2. Full name, 3. ID prefix. An error is returned if the ID-prefix match is ambiguous
	for _, service := range opts.services {
		for _, s := range serviceByIDList {
			if s.ID == service {
				filter.Add("service", s.ID)
				serviceCount++
				continue loop
			}
		}
		for _, s := range serviceByNameList {
			if s.Spec.Annotations.Name == service {
				filter.Add("service", s.ID)
				serviceCount++
				continue loop
			}
		}
		found := false
		for _, s := range serviceByIDList {
			if strings.HasPrefix(s.ID, service) {
				if found {
					return errors.New("multiple services found with provided prefix: " + service)
				}
				filter.Add("service", s.ID)
				serviceCount++
				found = true
			}
		}
		if !found {
			errs = append(errs, "no such service: "+service)
		}
	}
	if serviceCount == 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
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

	tasks, err := client.TaskList(ctx, types.TaskListOptions{Filters: filter})
	if err != nil {
		return err
	}

	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().TasksFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().TasksFormat
		} else {
			format = formatter.TableFormatKey
		}
	}
	if err := task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve), !opts.noTrunc, opts.quiet, format); err != nil {
		return err
	}
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
