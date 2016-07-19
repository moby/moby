package service

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
)

// NewServiceCommand returns a cobra command for `service` subcommands
func NewServiceCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage Docker services",
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
		},
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newInspectCommand(dockerCli),
		newPSCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
		newScaleCommand(dockerCli),
		newUpdateCommand(dockerCli),
	)
	return cmd
}
