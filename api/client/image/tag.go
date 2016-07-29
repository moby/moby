package image

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type tagOptions struct {
	image string
	name  string
}

// NewTagCommand create a new `docker tag` command
func NewTagCommand(dockerCli *client.DockerCli) *cobra.Command {
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
	// TODO remove dummy '--force' / '-f' flag for 1.13. It's only there for backward compatibility
	var forceTag bool
	flags.BoolVarP(&forceTag, "force", "f", false, "Force the tagging even if there's a conflict")
	flags.MarkDeprecated("force", "force tagging is now enabled by default. This flag will be removed in Docker 1.13")

	return cmd
}

func runTag(dockerCli *client.DockerCli, opts tagOptions) error {
	ctx := context.Background()

	return dockerCli.Client().ImageTag(ctx, opts.image, opts.name)
}
