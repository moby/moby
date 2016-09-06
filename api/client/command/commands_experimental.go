// +build experimental

package command

import (
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/system"
	"github.com/spf13/cobra"
)

func addExperimentalCommands(cmd *cobra.Command, dockerCli *client.DockerCli) {
	cmd.AddCommand(system.NewTunnelCommand(dockerCli))
}
