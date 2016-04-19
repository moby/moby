package volume

import (
	"github.com/spf13/cobra"

	"github.com/docker/docker/api/client"
)

// NewVolumeCommand returns a cobra command for `volume` subcommands
func NewVolumeCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage Docker volumes",
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newInspectCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
	)
	return cmd
}
