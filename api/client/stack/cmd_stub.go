// +build !experimental

package stack

import (
	"github.com/docker/docker/api/client"
	"github.com/spf13/cobra"
)

// NewStackCommand returns nocommand
func NewStackCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{}
}

// NewTopLevelDeployCommand return no command
func NewTopLevelDeployCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{}
}
