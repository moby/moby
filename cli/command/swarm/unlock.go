package swarm

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

func newUnlockCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock swarm",
		Args:  cli.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := dockerCli.Client()
			ctx := context.Background()

			key, err := readKey(dockerCli.In(), "Please enter unlock key: ")
			if err != nil {
				return err
			}
			req := swarm.UnlockRequest{
				LockKey: string(key),
			}

			return client.SwarmUnlock(ctx, req)
		},
	}

	return cmd
}
