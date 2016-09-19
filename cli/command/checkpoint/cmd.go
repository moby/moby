// +build !experimental

package checkpoint

import (
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewCheckpointCommand returns the `checkpoint` subcommand (only in experimental)
func NewCheckpointCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{}
}
