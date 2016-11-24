package network

import (
	"sort"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

type byNetworkName []types.NetworkResource

func (r byNetworkName) Len() int           { return len(r) }
func (r byNetworkName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byNetworkName) Less(i, j int) bool { return r[i].Name < r[j].Name }

type listOptions struct {
	quiet   bool
	noTrunc bool
	format  string
	filter  opts.FilterOpt
}

func newListCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List networks",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display network IDs")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate the output")
	flags.StringVar(&opts.format, "format", "", "Pretty-print networks using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Provide filter values (e.g. 'driver=bridge')")

	return cmd
}

func runList(dockerCli *command.DockerCli, opts listOptions) error {
	client := dockerCli.Client()
	options := types.NetworkListOptions{Filters: opts.filter.Value()}
	networkResources, err := client.NetworkList(context.Background(), options)
	if err != nil {
		return err
	}

	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().NetworksFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().NetworksFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	sort.Sort(byNetworkName(networkResources))

	networksCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewNetworkFormat(format, opts.quiet),
		Trunc:  !opts.noTrunc,
	}
	return formatter.NetworkWrite(networksCtx, networkResources)
}
