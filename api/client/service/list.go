package service

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

const (
	listItemFmt = "%s\t%s\t%s\t%s\t%s\n"
)

type listOptions struct {
	quiet  bool
	filter opts.FilterOpt
}

func newListCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List services",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display IDs")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runList(dockerCli *client.DockerCli, opts listOptions) error {
	client := dockerCli.Client()

	services, err := client.ServiceList(
		context.Background(),
		types.ServiceListOptions{Filter: opts.filter.Value()})
	if err != nil {
		return err
	}

	out := dockerCli.Out()
	if opts.quiet {
		printQuiet(out, services)
	} else {
		printTable(out, services)
	}
	return nil
}

func printTable(out io.Writer, services []swarm.Service) {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)

	// Ignore flushing errors
	defer writer.Flush()

	fmt.Fprintf(writer, listItemFmt, "ID", "NAME", "SCALE", "IMAGE", "COMMAND")
	for _, service := range services {
		scale := ""
		if service.Spec.Mode.Replicated != nil {
			scale = fmt.Sprintf("%d", *service.Spec.Mode.Replicated.Instances)
		} else if service.Spec.Mode.Global != nil {
			scale = "global"
		}
		fmt.Fprintf(
			writer,
			listItemFmt,
			stringid.TruncateID(service.ID),
			service.Spec.Name,
			scale,
			service.Spec.TaskSpec.ContainerSpec.Image,
			strings.Join(service.Spec.TaskSpec.ContainerSpec.Command, " "))
	}
}

func printQuiet(out io.Writer, services []swarm.Service) {
	for _, service := range services {
		fmt.Fprintln(out, service.ID)
	}
}
