// +build experimental

package plugin

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type pluginOptions struct {
	name       string
	grantPerms bool
	disable    bool
}

func newInstallCommand(dockerCli *command.DockerCli) *cobra.Command {
	var options pluginOptions
	cmd := &cobra.Command{
		Use:   "install [OPTIONS] PLUGIN",
		Short: "Install a plugin",
		Args:  cli.ExactArgs(1), // TODO: allow for set args
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return runInstall(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&options.grantPerms, "grant-all-permissions", false, "Grant all permissions necessary to run the plugin")
	flags.BoolVar(&options.disable, "disable", false, "Do not enable the plugin on install")

	return cmd
}

func runInstall(dockerCli *command.DockerCli, opts pluginOptions) error {
	named, err := reference.ParseNamed(opts.name) // FIXME: validate
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

	registryAuthFunc := command.RegistryAuthenticationPrivilegedFunc(dockerCli, repoInfo.Index, "plugin install")

	options := types.PluginInstallOptions{
		RegistryAuth:          encodedAuth,
		Disabled:              opts.disable,
		AcceptAllPermissions:  opts.grantPerms,
		AcceptPermissionsFunc: acceptPrivileges(dockerCli, opts.name),
		// TODO: Rename PrivilegeFunc, it has nothing to do with privileges
		PrivilegeFunc: registryAuthFunc,
	}
	if err := dockerCli.Client().PluginInstall(ctx, ref.String(), options); err != nil {
		return err
	}
	fmt.Fprintln(dockerCli.Out(), opts.name)
	return nil
}

func acceptPrivileges(dockerCli *command.DockerCli, name string) func(privileges types.PluginPrivileges) (bool, error) {
	return func(privileges types.PluginPrivileges) (bool, error) {
		fmt.Fprintf(dockerCli.Out(), "Plugin %q is requesting the following privileges:\n", name)
		for _, privilege := range privileges {
			fmt.Fprintf(dockerCli.Out(), " - %s: %v\n", privilege.Name, privilege.Value)
		}

		fmt.Fprint(dockerCli.Out(), "Do you grant the above permissions? [y/N] ")
		reader := bufio.NewReader(dockerCli.In())
		line, _, err := reader.ReadLine()
		if err != nil {
			return false, err
		}
		return strings.ToLower(string(line)) == "y", nil
	}
}
