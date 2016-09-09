// +build !experimental

package checkpoint

import (
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewCheckpointCommand appends the `checkpoint` subcommands to rootCmd (only in experimental)
func NewCheckpointCommand(rootCmd *cobra.Command, dockerCli *command.DockerCli) {
}
