// +build experimental

package plugin

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/reference"
	"github.com/spf13/cobra"
)

func newSetCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set PLUGIN key1=value1 [key2=value2...]",
		Short: "Change settings for a plugin",
		Args:  cli.RequiresMinArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSet(dockerCli, args[0], args[1:])
		},
	}

	return cmd
}

func runSet(dockerCli *command.DockerCli, name string, args []string) error {
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
	return dockerCli.Client().PluginSet(context.Background(), ref.String(), args)
}
