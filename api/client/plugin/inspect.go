// +build experimental

package plugin

import (
	"encoding/json"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(dockerCli, args[0])
		},
	}

	return cmd
}

func runInspect(dockerCli *client.DockerCli, name string) error {
	p, err := dockerCli.Client().PluginInspect(context.Background(), name)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}
	_, err = dockerCli.Out().Write(b)
	return err
}
