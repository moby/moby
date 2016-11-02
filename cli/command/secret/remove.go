package secret

import (
	"context"
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	ids []string
}

func newSecretRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "rm [id]",
		Short: "Remove a secret",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := removeOptions{
				ids: args,
			}
			return runSecretRemove(dockerCli, opts)
		},
	}
}

func runSecretRemove(dockerCli *command.DockerCli, opts removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	// attempt to lookup secret by name
	secrets, err := getSecrets(client, ctx, opts.ids)
	if err != nil {
		return err
	}

	ids := opts.ids

	names := make(map[string]int)
	for _, id := range ids {
		names[id] = 1
	}

	if len(secrets) > 0 {
		ids = []string{}

		for _, s := range secrets {
			if _, ok := names[s.Spec.Annotations.Name]; ok {
				ids = append(ids, s.ID)
			}
		}
	}

	for _, id := range ids {
		if err := client.SecretRemove(ctx, id); err != nil {
			return err
		}

		fmt.Fprintln(dockerCli.Out(), id)
	}

	return nil
}
