// +build !experimental

package stack

import (
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewStackCommand returns no command
func NewStackCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{}
}

// NewTopLevelDeployCommand returns no command
func NewTopLevelDeployCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{}
}
