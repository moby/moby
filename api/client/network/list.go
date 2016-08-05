package network

import (
	"sort"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/formatter"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
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
	filter  []string
}

func newListCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts listOptions

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
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display volume names")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate the output")
	flags.StringVar(&opts.format, "format", "", "Pretty-print networks using a Go template")
	flags.StringSliceVarP(&opts.filter, "filter", "f", []string{}, "Provide filter values (i.e. 'dangling=true')")

	return cmd
}

func runList(dockerCli *client.DockerCli, opts listOptions) error {
	client := dockerCli.Client()

	netFilterArgs := filters.NewArgs()
	for _, f := range opts.filter {
		var err error
		netFilterArgs, err = filters.ParseFlag(f, netFilterArgs)
		if err != nil {
			return err
		}
	}

	options := types.NetworkListOptions{
		Filters: netFilterArgs,
	}

	networkResources, err := client.NetworkList(context.Background(), options)
	if err != nil {
		return err
	}

	f := opts.format
	if len(f) == 0 {
		if len(dockerCli.ConfigFile().NetworksFormat) > 0 && !opts.quiet {
			f = dockerCli.ConfigFile().NetworksFormat
		} else {
			f = "table"
		}
	}

	sort.Sort(byNetworkName(networkResources))

	networksCtx := formatter.NetworkContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: f,
			Quiet:  opts.quiet,
			Trunc:  !opts.noTrunc,
		},
		Networks: networkResources,
	}

	networksCtx.Write()

	return nil
}
