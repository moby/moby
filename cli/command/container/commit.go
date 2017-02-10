package container

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	dockeropts "github.com/docker/docker/opts"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type commitOptions struct {
	container string
	reference string

	pause   bool
	comment string
	author  string
	changes dockeropts.ListOpts
}

// NewCommitCommand creates a new cobra.Command for `docker commit`
func NewCommitCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts commitOptions

	cmd := &cobra.Command{
		Use:   "commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]",
		Short: "Create a new image from a container's changes",
		Args:  cli.RequiresRangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			if len(args) > 1 {
				opts.reference = args[1]
			}
			return runCommit(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.BoolVarP(&opts.pause, "pause", "p", true, "Pause container during commit")
	flags.StringVarP(&opts.comment, "message", "m", "", "Commit message")
	flags.StringVarP(&opts.author, "author", "a", "", "Author (e.g., \"John Hannibal Smith <hannibal@a-team.com>\")")

	opts.changes = dockeropts.NewListOpts(nil)
	flags.VarP(&opts.changes, "change", "c", "Apply Dockerfile instruction to the created image")

	return cmd
}

func runCommit(dockerCli *command.DockerCli, opts *commitOptions) error {
	ctx := context.Background()

	name := opts.container
	reference := opts.reference

	options := types.ContainerCommitOptions{
		Reference: reference,
		Comment:   opts.comment,
		Author:    opts.author,
		Changes:   opts.changes.GetAll(),
		Pause:     opts.pause,
	}

	response, err := dockerCli.Client().ContainerCommit(ctx, name, options)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), response.ID)
	return nil
}
