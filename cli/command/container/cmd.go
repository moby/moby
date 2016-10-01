package container

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewContainerCommand returns a cobra command for `container` subcommands
func NewContainerCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container",
		Short: "Manage containers",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
		},
	}
	cmd.AddCommand(
		NewAttachCommand(dockerCli),
		NewCommitCommand(dockerCli),
		NewCopyCommand(dockerCli),
		NewCreateCommand(dockerCli),
		NewDiffCommand(dockerCli),
		NewExecCommand(dockerCli),
		NewExportCommand(dockerCli),
		NewKillCommand(dockerCli),
		NewLogsCommand(dockerCli),
		NewPauseCommand(dockerCli),
		NewPortCommand(dockerCli),
		NewRenameCommand(dockerCli),
		NewRestartCommand(dockerCli),
		NewRmCommand(dockerCli),
		NewRunCommand(dockerCli),
		NewStartCommand(dockerCli),
		NewStatsCommand(dockerCli),
		NewStopCommand(dockerCli),
		NewTopCommand(dockerCli),
		NewUnpauseCommand(dockerCli),
		NewUpdateCommand(dockerCli),
		NewWaitCommand(dockerCli),
		newListCommand(dockerCli),
		newInspectCommand(dockerCli),
		NewPruneCommand(dockerCli),
	)
	return cmd
}
