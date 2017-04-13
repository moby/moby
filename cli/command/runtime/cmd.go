package runtime

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewRuntimeCommand returns a cobra command for `runtime` subcommands
func NewRuntimeCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime COMMAND",
		Short: "Manage runtimes",
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
		Tags:  map[string]string{"version": "1.21"},
	}
	cmd.AddCommand(
		newListCommand(dockerCli),
		newDefaultCommand(dockerCli),
	)
	return cmd
}
