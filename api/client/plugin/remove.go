// +build experimental

package plugin

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newRemoveCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm",
		Short:   "Remove a plugin",
		Aliases: []string{"remove"},
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args)
		},
	}

	return cmd
}

func runRemove(dockerCli *client.DockerCli, names []string) error {
	var errs cli.Errors
	for _, name := range names {
		// TODO: pass names to api instead of making multiple api calls
		if err := dockerCli.Client().PluginRemove(context.Background(), name); err != nil {
			errs = append(errs, err)
			continue
		}
		fmt.Fprintln(dockerCli.Out(), name)
	}
	// Do not simplify to `return errs` because even if errs == nil, it is not a nil-error interface value.
	if errs != nil {
		return errs
	}
	return nil
}
