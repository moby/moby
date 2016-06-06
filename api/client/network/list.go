package network

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/stringid"
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
	filter  []string
}

func newListCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "ls",
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

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)
	if !opts.quiet {
		fmt.Fprintf(w, "NETWORK ID\tNAME\tDRIVER")
		fmt.Fprintf(w, "\n")
	}

	sort.Sort(byNetworkName(networkResources))
	for _, networkResource := range networkResources {
		ID := networkResource.ID
		netName := networkResource.Name
		if !opts.noTrunc {
			ID = stringid.TruncateID(ID)
		}
		if opts.quiet {
			fmt.Fprintln(w, ID)
			continue
		}
		driver := networkResource.Driver
		fmt.Fprintf(w, "%s\t%s\t%s\t",
			ID,
			netName,
			driver)
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}
