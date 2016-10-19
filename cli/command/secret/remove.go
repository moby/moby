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

	for _, id := range opts.ids {
		if err := client.SecretRemove(ctx, id); err != nil {
			return err
		}

		fmt.Fprintln(dockerCli.Out(), id)
	}

	return nil
}
