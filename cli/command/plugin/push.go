// +build experimental

package plugin

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
)

func newPushCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push PLUGIN",
		Short: "Push a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPush(dockerCli, args[0])
		},
	}
	return cmd
}

func runPush(dockerCli *command.DockerCli, name string) error {
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

	ctx := context.Background()

	repoInfo, err := registry.ParseRepositoryInfo(named)
	if err != nil {
		return err
	}
	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}
	return dockerCli.Client().PluginPush(ctx, ref.String(), encodedAuth)
}
