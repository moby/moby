// +build experimental

package plugin

import (
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/reference"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newEnableCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable PLUGIN",
		Short: "Enable a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnable(dockerCli, args[0])
		},
	}

	return cmd
}

func runEnable(dockerCli *command.DockerCli, name string) error {
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
	if err := dockerCli.Client().PluginEnable(context.Background(), ref.String()); err != nil {
		return err
	}
	fmt.Fprintln(dockerCli.Out(), name)
	return nil
}
