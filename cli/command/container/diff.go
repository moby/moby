package container

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/pkg/archive"
	"github.com/spf13/cobra"
)

type diffOptions struct {
	container string
}

// NewDiffCommand creates a new cobra.Command for `docker diff`
func NewDiffCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts diffOptions

	return &cobra.Command{
		Use:   "diff CONTAINER",
		Short: "Inspect changes on a container's filesystem",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runDiff(dockerCli, &opts)
		},
	}
}

func runDiff(dockerCli *command.DockerCli, opts *diffOptions) error {
	if opts.container == "" {
		return fmt.Errorf("Container name cannot be empty")
	}
	ctx := context.Background()

	changes, err := dockerCli.Client().ContainerDiff(ctx, opts.container)
	if err != nil {
		return err
	}

	for _, change := range changes {
		var kind string
		switch change.Kind {
		case archive.ChangeModify:
			kind = "C"
		case archive.ChangeAdd:
			kind = "A"
		case archive.ChangeDelete:
			kind = "D"
		}
		fmt.Fprintf(dockerCli.Out(), "%s %s\n", kind, change.Path)
	}

	return nil
}
