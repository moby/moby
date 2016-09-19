// +build !experimental

package plugin

import (
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewPluginCommand returns a cobra command for `plugin` subcommands
func NewPluginCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{}
}
