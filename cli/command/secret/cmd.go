package secret

import (
	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewSecretCommand returns a cobra command for `secret` subcommands
func NewSecretCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage Docker secrets",
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
	}
	cmd.AddCommand(
		newSecretListCommand(dockerCli),
		newSecretCreateCommand(dockerCli),
		newSecretInspectCommand(dockerCli),
		newSecretRemoveCommand(dockerCli),
	)
	return cmd
}
