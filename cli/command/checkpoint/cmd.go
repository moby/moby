package checkpoint

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewCheckpointCommand returns the `checkpoint` subcommand (only in experimental)
func NewCheckpointCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage checkpoints",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.SetOutput(dockerCli.Err())
			cmd.HelpFunc()(cmd, args)
		},
		Tags: map[string]string{"experimental": "", "version": "1.25"},
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
	)
	return cmd
}
