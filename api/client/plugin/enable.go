// +build experimental

package plugin

import (
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newEnableCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable PLUGIN",
		Short: "Enable a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dockerCli.Client().PluginEnable(context.Background(), args[0])
		},
	}

	return cmd
}
