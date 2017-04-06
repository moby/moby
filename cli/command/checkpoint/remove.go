package checkpoint

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	checkpointDir string
}

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] CONTAINER CHECKPOINT",
		Aliases: []string{"remove"},
		Short:   "Remove a checkpoint",
		Args:    cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args[0], args[1], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.checkpointDir, "checkpoint-dir", "", "", "Use a custom checkpoint storage directory")

	return cmd
}

func runRemove(dockerCli *command.DockerCli, container string, checkpoint string, opts removeOptions) error {
	client := dockerCli.Client()

	removeOpts := types.CheckpointDeleteOptions{
		CheckpointID:  checkpoint,
		CheckpointDir: opts.checkpointDir,
	}

	return client.CheckpointDelete(context.Background(), container, removeOpts)
}
