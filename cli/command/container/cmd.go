package container

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewContainerCommand returns a cobra command for `container` subcommands
func NewContainerCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container",
		Short: "Manage containers",
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
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
