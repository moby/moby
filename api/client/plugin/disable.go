// +build experimental

package plugin

import (
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newDisableCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dockerCli.Client().PluginDisable(context.Background(), args[0])
		},
	}

	return cmd
}
