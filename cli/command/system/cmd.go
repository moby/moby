package system

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewSystemCommand returns a cobra command for `system` subcommands
func NewSystemCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage Docker",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
		},
	}
	cmd.AddCommand(
		NewEventsCommand(dockerCli),
		NewInfoCommand(dockerCli),
		NewDiskUsageCommand(dockerCli),
		NewPruneCommand(dockerCli),
	)
	return cmd
}
