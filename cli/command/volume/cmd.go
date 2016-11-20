package volume

import (
	"github.com/spf13/cobra"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
)

// NewVolumeCommand returns a cobra command for `volume` subcommands
func NewVolumeCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume COMMAND",
		Short: "Manage volumes",
		Long:  volumeDescription,
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newInspectCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
		NewPruneCommand(dockerCli),
	)
	return cmd
}

var volumeDescription = `
The **docker volume** command has subcommands for managing data volumes. A data
volume is a specially-designated directory that by-passes storage driver
management.

Data volumes persist data independent of a container's life cycle. When you
delete a container, the Docker daemon does not delete any data volumes. You can
share volumes across multiple containers. Moreover, you can share data volumes
with other computing resources in your system.

To see help for a subcommand, use:

    docker volume COMMAND --help

For full details on using docker volume visit Docker's online documentation.

`
