package plugin

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/image"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type pluginOptions struct {
	name       string
	alias      string
	grantPerms bool
	disable    bool
	args       []string
}

func newInstallCommand(dockerCli *command.DockerCli) *cobra.Command {
	var options pluginOptions
	cmd := &cobra.Command{
		Use:   "install [OPTIONS] PLUGIN [KEY=VALUE...]",
		Short: "Install a plugin",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			if len(args) > 1 {
				options.args = args[1:]
			}
			return runInstall(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&options.grantPerms, "grant-all-permissions", false, "Grant all permissions necessary to run the plugin")
	flags.BoolVar(&options.disable, "disable", false, "Do not enable the plugin on install")
	flags.StringVar(&options.alias, "alias", "", "Local name for plugin")

	command.AddTrustVerificationFlags(flags)

	return cmd
}

func getRepoIndexFromUnnormalizedRef(ref reference.Named) (*registrytypes.IndexInfo, error) {
	named, err := reference.ParseNormalizedNamed(ref.Name())
	if err != nil {
		return nil, err
	}

	repoInfo, err := registry.ParseRepositoryInfo(named)
	if err != nil {
		return nil, err
	}

	return repoInfo.Index, nil
}

type pluginRegistryService struct {
	registry.Service
}

func (s pluginRegistryService) ResolveRepository(name reference.Named) (repoInfo *registry.RepositoryInfo, err error) {
	repoInfo, err = s.Service.ResolveRepository(name)
	if repoInfo != nil {
		repoInfo.Class = "plugin"
	}
	return
}

func newRegistryService() registry.Service {
	return pluginRegistryService{
		Service: registry.NewService(registry.ServiceOptions{V2Only: true}),
	}
}

func runInstall(dockerCli *command.DockerCli, opts pluginOptions) error {
	// Names with both tag and digest will be treated by the daemon
	// as a pull by digest with an alias for the tag
	// (if no alias is provided).
	ref, err := reference.ParseNormalizedNamed(opts.name)
	if err != nil {
		return err
	}

	alias := ""
	if opts.alias != "" {
		aref, err := reference.ParseNormalizedNamed(opts.alias)
		if err != nil {
			return err
		}
		if _, ok := aref.(reference.Canonical); ok {
			return fmt.Errorf("invalid name: %s", opts.alias)
		}
		alias = reference.FamiliarString(reference.EnsureTagged(aref))
	}
	ctx := context.Background()

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}

	remote := ref.String()

	_, isCanonical := ref.(reference.Canonical)
	if command.IsTrusted() && !isCanonical {
		if alias == "" {
			alias = reference.FamiliarString(ref)
		}

		nt, ok := ref.(reference.NamedTagged)
		if !ok {
			nt = reference.EnsureTagged(ref)
		}

		trusted, err := image.TrustedReference(ctx, dockerCli, nt, newRegistryService())
		if err != nil {
			return err
		}
		remote = reference.FamiliarString(trusted)
	}

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	registryAuthFunc := command.RegistryAuthenticationPrivilegedFunc(dockerCli, repoInfo.Index, "plugin install")

	options := types.PluginInstallOptions{
		RegistryAuth:          encodedAuth,
		RemoteRef:             remote,
		Disabled:              opts.disable,
		AcceptAllPermissions:  opts.grantPerms,
		AcceptPermissionsFunc: acceptPrivileges(dockerCli, opts.name),
		// TODO: Rename PrivilegeFunc, it has nothing to do with privileges
		PrivilegeFunc: registryAuthFunc,
		Args:          opts.args,
	}

	responseBody, err := dockerCli.Client().PluginInstall(ctx, alias, options)
	if err != nil {
		if strings.Contains(err.Error(), "target is image") {
			return errors.New(err.Error() + " - Use `docker image pull`")
		}
		return err
	}
	defer responseBody.Close()
	if err := jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil); err != nil {
		return err
	}
	fmt.Fprintf(dockerCli.Out(), "Installed plugin %s\n", opts.name) // todo: return proper values from the API for this result
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
