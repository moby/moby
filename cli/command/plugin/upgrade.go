package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newUpgradeCommand(dockerCli *command.DockerCli) *cobra.Command {
	var options pluginOptions
	cmd := &cobra.Command{
		Use:   "upgrade [OPTIONS] PLUGIN [REMOTE]",
		Short: "Upgrade an existing plugin",
		Args:  cli.RequiresRangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.localName = args[0]
			if len(args) == 2 {
				options.remote = args[1]
			}
			return runUpgrade(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	loadPullFlags(&options, flags)
	flags.BoolVar(&options.skipRemoteCheck, "skip-remote-check", false, "Do not check if specified remote plugin matches existing plugin image")
	return cmd
}

func runUpgrade(dockerCli *command.DockerCli, opts pluginOptions) error {
	ctx := context.Background()
	p, _, err := dockerCli.Client().PluginInspectWithRaw(ctx, opts.localName)
	if err != nil {
		return fmt.Errorf("error reading plugin data: %v", err)
	}

	if p.Enabled {
		return fmt.Errorf("the plugin must be disabled before upgrading")
	}

	opts.localName = p.Name
	if opts.remote == "" {
		opts.remote = p.PluginReference
	}
	remote, err := reference.ParseNormalizedNamed(opts.remote)
	if err != nil {
		return errors.Wrap(err, "error parsing remote upgrade image reference")
	}
	remote = reference.TagNameOnly(remote)

	old, err := reference.ParseNormalizedNamed(p.PluginReference)
	if err != nil {
		return errors.Wrap(err, "error parsing current image reference")
	}
	old = reference.TagNameOnly(old)

	fmt.Fprintf(dockerCli.Out(), "Upgrading plugin %s from %s to %s\n", p.Name, reference.FamiliarString(old), reference.FamiliarString(remote))
	if !opts.skipRemoteCheck && remote.String() != old.String() {
		if !command.PromptForConfirmation(dockerCli.In(), dockerCli.Out(), "Plugin images do not match, are you sure?") {
			return errors.New("canceling upgrade request")
		}
	}

	options, err := buildPullConfig(ctx, dockerCli, opts, "plugin upgrade")
	if err != nil {
		return err
	}

	responseBody, err := dockerCli.Client().PluginUpgrade(ctx, opts.localName, options)
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
	fmt.Fprintf(dockerCli.Out(), "Upgraded plugin %s to %s\n", opts.localName, opts.remote) // todo: return proper values from the API for this result
	return nil
}
