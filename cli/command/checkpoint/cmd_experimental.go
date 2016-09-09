// +build experimental

package checkpoint

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewCheckpointCommand appends the `checkpoint` subcommands to rootCmd
func NewCheckpointCommand(rootCmd *cobra.Command, dockerCli *command.DockerCli) {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage Container Checkpoints",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
		},
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
	)

	rootCmd.AddCommand(cmd)
}
