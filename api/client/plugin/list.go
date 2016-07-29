// +build experimental

package plugin

import (
	"fmt"
	"text/tabwriter"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newListCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ls",
		Short:   "List plugins",
		Aliases: []string{"list"},
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli)
		},
	}

	return cmd
}

func runList(dockerCli *client.DockerCli) error {
	plugins, err := dockerCli.Client().PluginList(context.Background())
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME \tTAG \tACTIVE")
	fmt.Fprintf(w, "\n")

	for _, p := range plugins {
		fmt.Fprintf(w, "%s\t%s\t%v\n", p.Name, p.Tag, p.Active)
	}
	w.Flush()
	return nil
}
