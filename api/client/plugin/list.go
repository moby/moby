// +build experimental

package plugin

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type listOptions struct {
	noTrunc bool
}

func newListCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "ls",
		Short:   "List plugins",
		Aliases: []string{"list"},
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Don't truncate output")

	return cmd
}

func runList(dockerCli *client.DockerCli, opts listOptions) error {
	plugins, err := dockerCli.Client().PluginList(context.Background())
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME \tTAG \tDESCRIPTION\tACTIVE")
	fmt.Fprintf(w, "\n")

	for _, p := range plugins {
		desc := strings.Replace(p.Manifest.Description, "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !opts.noTrunc {
			desc = stringutils.Ellipsis(desc, 45)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", p.Name, p.Tag, desc, p.Active)
	}
	w.Flush()
	return nil
}
