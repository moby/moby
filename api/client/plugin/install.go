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

type pluginOptions struct {
	name       string
	grantPerms bool
}

func newInstallCommand(dockerCli *client.DockerCli) *cobra.Command {
	var options pluginOptions
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install a plugin",
		Args:  cli.RequiresMinArgs(1), // TODO: allow for set args
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return runInstall(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&options.grantPerms, "grant-all-permissions", true, "grant all permissions necessary to run the plugin")

	return cmd
}

func runInstall(dockerCli *client.DockerCli, options pluginOptions) error {
	named, err := reference.ParseNamed(options.name) // FIXME: validate
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
	// TODO: pass noEnable flag
	return dockerCli.Client().PluginInstall(ctx, ref.String(), encodedAuth, options.grantPerms, false, dockerCli.In(), dockerCli.Out())
}
