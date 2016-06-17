// +build experimental

package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/reference"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect PLUGIN",
		Short: "Inspect a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(dockerCli, args[0])
		},
	}

	return cmd
}

func runInspect(dockerCli *client.DockerCli, name string) error {
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
	p, err := dockerCli.Client().PluginInspect(context.Background(), ref.String())
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
