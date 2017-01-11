package volume

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	force bool

	volumes []string
}

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] VOLUME [VOLUME...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more volumes",
		Long:    removeDescription,
		Example: removeExample,
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.volumes = args
			return runRemove(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Force the removal of one or more volumes")
	flags.SetAnnotation("force", "version", []string{"1.25"})
	return cmd
}

func runRemove(dockerCli *command.DockerCli, opts *removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	status := 0

	for _, name := range opts.volumes {
		if err := client.VolumeRemove(ctx, name, opts.force); err != nil {
			fmt.Fprintf(dockerCli.Err(), "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", name)
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}
	return nil
}

var removeDescription = `
Remove one or more volumes. You cannot remove a volume that is in use by a container.
`

var removeExample = `
$ docker volume rm hello
hello
`
