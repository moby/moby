package manifest

import (
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

// NewManifestCommand returns a cobra command for `manifest` subcommands
func NewManifestCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest COMMAND",
		Short: "Manage Docker image manifests and lists",
		Long:  manifestDescription,
		Args:  cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
		},
	}
	cmd.AddCommand(
		//newListFetchCommand(dockerCli),
		newCreateListCommand(dockerCli),
		newInspectCommand(dockerCli),
		newAnnotateCommand(dockerCli),
		newPushListCommand(dockerCli),
	)
	return cmd
}

var manifestDescription = `
The **docker manifest** command has subcommands for managing image manifests and 
manifest lists. A manifest list allows you to use one name to refer to the same image 
built for multiple architectures.

To see help for a subcommand, use:

    docker manifest CMD help

For full details on using docker manifest lists view the registry v2 specification.

`
