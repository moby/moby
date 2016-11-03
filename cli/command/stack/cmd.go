package stack

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewStackCommand returns a cobra command for `stack` subcommands
func NewStackCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage Docker stacks",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.SetOutput(dockerCli.Err())
			cmd.HelpFunc()(cmd, args)
		},
		Tags: map[string]string{"experimental": "", "version": "1.25"},
	}
	cmd.AddCommand(
		newConfigCommand(dockerCli),
		newDeployCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
		newServicesCommand(dockerCli),
		newPsCommand(dockerCli),
	)
	return cmd
}

// NewTopLevelDeployCommand returns a command for `docker deploy`
func NewTopLevelDeployCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := newDeployCommand(dockerCli)
	// Remove the aliases at the top level
	cmd.Aliases = []string{}
	return cmd
}
