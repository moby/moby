package volume

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
)

// NewVolumeCommand returns a cobra command for `volume` subcommands
func NewVolumeCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage Docker volumes",
		// TODO: remove once cobra is patched to handle this
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(dockerCli.Err(), "\n%s", cmd.UsageString())
			if len(args) > 0 {
				return cli.StatusError{StatusCode: 1}
			}
			return nil
		},
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newInspectCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
	)
	return cmd
}
