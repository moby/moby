package container

import (
	"encoding/json"
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	dockeropts "github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/spf13/cobra"
)

type commitOptions struct {
	container string
	reference string

	pause   bool
	comment string
	author  string
	changes dockeropts.ListOpts
	config  string
}

// NewCommitCommand creats a new cobra.Command for `docker commit`
func NewCommitCommand(dockerCli *client.DockerCli) *cobra.Command {
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

	// FIXME: --run is deprecated, it will be replaced with inline Dockerfile commands.
	flags.StringVar(&opts.config, "run", "", "This option is deprecated and will be removed in a future version in favor of inline Dockerfile-compatible commands")
	flags.MarkDeprecated("run", "it will be replaced with inline Dockerfile commands.")

	return cmd
}

func runCommit(dockerCli *client.DockerCli, opts *commitOptions) error {
	ctx := context.Background()

	name := opts.container
	reference := opts.reference

	var config *containertypes.Config
	if opts.config != "" {
		config = &containertypes.Config{}
		if err := json.Unmarshal([]byte(opts.config), config); err != nil {
			return err
		}
	}

	options := types.ContainerCommitOptions{
		Reference: reference,
		Comment:   opts.comment,
		Author:    opts.author,
		Changes:   opts.changes.GetAll(),
		Pause:     opts.pause,
		Config:    config,
	}

	response, err := dockerCli.Client().ContainerCommit(ctx, name, options)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), response.ID)
	return nil
}
