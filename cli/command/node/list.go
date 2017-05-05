package node

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

type listOptions struct {
	quiet  bool
	format string
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
	flags.StringVar(&opts.format, "format", "", "Pretty-print nodes using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runList(dockerCli command.Cli, opts listOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	nodes, err := client.NodeList(
		ctx,
		types.NodeListOptions{Filters: opts.filter.Value()})
	if err != nil {
		return err
	}

	info := types.Info{}
	if len(nodes) > 0 && !opts.quiet {
		// only non-empty nodes and not quiet, should we call /info api
		info, err = client.Info(ctx)
		if err != nil {
			return err
		}
	}

	format := opts.format
	if len(format) == 0 {
		format = formatter.TableFormatKey
		if len(dockerCli.ConfigFile().NodesFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().NodesFormat
		}
	}

	nodesCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewNodeFormat(format, opts.quiet),
	}
	return formatter.NodeWrite(nodesCtx, nodes, info)
}
