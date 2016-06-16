// +build experimental

package plugin

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newInstallCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install a plugin",
		Args:  cli.RequiresMinArgs(1), // TODO: allow for set args
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(dockerCli, args[0], args[1:])
		},
	}

	return cmd
}

func runInstall(dockerCli *client.DockerCli, name string, args []string) error {
	named, err := reference.ParseNamed(name) // FIXME: validate
	if err != nil {
		return err
	}
	named = reference.WithDefaultTag(named)
	ref, ok := named.(reference.NamedTagged)
	if !ok {
		return fmt.Errorf("invalid name: %s", named.String())
	}

	ctx := context.Background()

	repoInfo, err := registry.ParseRepositoryInfo(named)
	authConfig := dockerCli.ResolveAuthConfig(ctx, repoInfo.Index)

	encodedAuth, err := client.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}
	// TODO: pass acceptAllPermissions and noEnable flag
	return dockerCli.Client().PluginInstall(ctx, ref.String(), encodedAuth, false, false, dockerCli.In(), dockerCli.Out())
}
