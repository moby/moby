// +build !experimental

package stack

import (
	"github.com/docker/docker/api/client"
	"github.com/spf13/cobra"
)

// NewStackCommand returns no command
func NewStackCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{}
}

// NewTopLevelDeployCommand returns no command
func NewTopLevelDeployCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{}
}
