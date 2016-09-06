// +build !experimental

package command

import (
	"github.com/docker/docker/api/client"
	"github.com/spf13/cobra"
)

func addExperimentalCommands(cmd *cobra.Command, dockerCli *client.DockerCli) {
}
