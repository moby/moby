package plugin

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/reference"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newDisableCommand(dockerCli *command.DockerCli) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "disable PLUGIN",
		Short: "Disable a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDisable(dockerCli, args[0], force)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Force the disable of an active plugin")
	return cmd
}

func runDisable(dockerCli *command.DockerCli, name string, force bool) error {
	named, err := reference.ParseNamed(name) // FIXME: validate
	if err != nil {
		return err
	}
	if reference.IsNameOnly(named) {
		named = reference.WithDefaultTag(named)
	}
	ref, ok := named.(reference.NamedTagged)
	if !ok {
		return fmt.Errorf("invalid name: %s", named.String())
	}
	if err := dockerCli.Client().PluginDisable(context.Background(), ref.String(), types.PluginDisableOptions{Force: force}); err != nil {
		return err
	}
	fmt.Fprintln(dockerCli.Out(), name)
	return nil
}
