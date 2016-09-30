package image

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type tagOptions struct {
	image string
	name  string
}

// NewTagCommand creates a new `docker tag` command
func NewTagCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts tagOptions

	cmd := &cobra.Command{
		Use:   "tag IMAGE[:TAG] IMAGE[:TAG]",
		Short: "Tag an image into a repository",
		Args:  cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.image = args[0]
			opts.name = args[1]
			return runTag(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	return cmd
}

func runTag(dockerCli *command.DockerCli, opts tagOptions) error {
	ctx := context.Background()

	return dockerCli.Client().ImageTag(ctx, opts.image, opts.name)
}
