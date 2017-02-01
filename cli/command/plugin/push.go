package plugin

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/image"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
)

func newPushCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push [OPTIONS] PLUGIN[:TAG]",
		Short: "Push a plugin to a registry",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPush(dockerCli, args[0])
		},
	}

	flags := cmd.Flags()

	command.AddTrustSigningFlags(flags)

	return cmd
}

func runPush(dockerCli *command.DockerCli, name string) error {
	named, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return err
	}
	if _, ok := named.(reference.Canonical); ok {
		return fmt.Errorf("invalid name: %s", name)
	}

	taggedRef, ok := named.(reference.NamedTagged)
	if !ok {
		taggedRef = reference.EnsureTagged(named)
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

	responseBody, err := dockerCli.Client().PluginPush(ctx, reference.FamiliarString(taggedRef), encodedAuth)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if command.IsTrusted() {
		repoInfo.Class = "plugin"
		return image.PushTrustedReference(dockerCli, repoInfo, named, authConfig, responseBody)
	}

	return jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil)
}
