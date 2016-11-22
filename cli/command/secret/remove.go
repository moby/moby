package secret

import (
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type removeOptions struct {
	names []string
}

func newSecretRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "rm SECRET [SECRET]",
		Short: "Remove a secret",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := removeOptions{
				names: args,
			}
			return runSecretRemove(dockerCli, opts)
		},
	}
}

func runSecretRemove(dockerCli *command.DockerCli, opts removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	ids, err := getCliRequestedSecretIDs(ctx, client, opts.names)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if err := client.SecretRemove(ctx, id); err != nil {
			fmt.Fprintf(dockerCli.Out(), "WARN: %s\n", err)
		}

		fmt.Fprintln(dockerCli.Out(), id)
	}

	return nil
}
