package node

import (
	"fmt"
	"io"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

const (
	listItemFmt = "%s\t%s\t%s\t%s\t%s\n"
)

type listOptions struct {
	quiet  bool
	filter opts.FilterOpt
}

func newListCommand(dockerCli command.Cli) *cobra.Command {
	opts := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List nodes in the swarm",
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

func runList(dockerCli command.Cli, opts listOptions) error {
	client := dockerCli.Client()
	out := dockerCli.Out()
	ctx := context.Background()

	nodes, err := client.NodeList(
		ctx,
		types.NodeListOptions{Filters: opts.filter.Value()})
	if err != nil {
		return err
	}

	if len(nodes) > 0 && !opts.quiet {
		// only non-empty nodes and not quiet, should we call /info api
		info, err := client.Info(ctx)
		if err != nil {
			return err
		}
		printTable(out, nodes, info)
	} else if !opts.quiet {
		// no nodes and not quiet, print only one line with columns ID, HOSTNAME, ...
		printTable(out, nodes, types.Info{})
	} else {
		printQuiet(out, nodes)
	}

	return nil
}

func printTable(out io.Writer, nodes []swarm.Node, info types.Info) {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)

	// Ignore flushing errors
	defer writer.Flush()

	fmt.Fprintf(writer, listItemFmt, "ID", "HOSTNAME", "STATUS", "AVAILABILITY", "MANAGER STATUS")
	for _, node := range nodes {
		name := node.Description.Hostname
		availability := string(node.Spec.Availability)

		reachability := ""
		if node.ManagerStatus != nil {
			if node.ManagerStatus.Leader {
				reachability = "Leader"
			} else {
				reachability = string(node.ManagerStatus.Reachability)
			}
		}

		ID := node.ID
		if node.ID == info.Swarm.NodeID {
			ID = ID + " *"
		}

		fmt.Fprintf(
			writer,
			listItemFmt,
			ID,
			name,
			command.PrettyPrint(string(node.Status.State)),
			command.PrettyPrint(availability),
			command.PrettyPrint(reachability))
	}
}

func printQuiet(out io.Writer, nodes []swarm.Node) {
	for _, node := range nodes {
		fmt.Fprintln(out, node.ID)
	}
}
