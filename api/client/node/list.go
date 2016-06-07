package node

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
	listItemFmt = "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
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

func runList(dockerCli *client.DockerCli, opts listOptions) error {
	client := dockerCli.Client()

	nodes, err := client.NodeList(
		context.Background(),
		types.NodeListOptions{Filter: opts.filter.Value()})
	if err != nil {
		return err
	}

	info, err := client.Info(context.Background())
	if err != nil {
		return err
	}

	out := dockerCli.Out()
	if opts.quiet {
		printQuiet(out, nodes)
	} else {
		printTable(out, nodes, info)
	}
	return nil
}

func printTable(out io.Writer, nodes []swarm.Node, info types.Info) {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	prettyPrint := func(s string) string {
		return strings.Title(strings.ToLower(s))
	}

	// Ignore flushing errors
	defer writer.Flush()

	fmt.Fprintf(writer, listItemFmt, "ID", "NAME", "MEMBERSHIP", "STATUS", "AVAILABILITY", "MANAGER STATUS", "LEADER")
	for _, node := range nodes {
		name := node.Spec.Name
		availability := string(node.Spec.Availability)
		membership := string(node.Spec.Membership)

		if name == "" {
			name = node.Description.Hostname
		}

		leader := ""
		if node.Manager != nil && node.Manager.Raft.Status.Leader {
			leader = "Yes"
		}

		reachability := ""
		if node.Manager != nil {
			reachability = string(node.Manager.Raft.Status.Reachability)
		}

		ID := stringid.TruncateID(node.ID)
		if node.Description.Hostname == info.Name {
			ID = ID + " *"
		}

		fmt.Fprintf(
			writer,
			listItemFmt,
			ID,
			name,
			prettyPrint(membership),
			prettyPrint(string(node.Status.State)),
			prettyPrint(availability),
			prettyPrint(reachability),
			leader)
	}
}

func printQuiet(out io.Writer, nodes []swarm.Node) {
	for _, node := range nodes {
		fmt.Fprintln(out, node.ID)
	}
}
