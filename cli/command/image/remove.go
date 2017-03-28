package image

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	force   bool
	noPrune bool
}

// NewRemoveCommand creates a new `docker remove` command
func NewRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:   "rmi [OPTIONS] IMAGE [IMAGE...]",
		Short: "Remove one or more images",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, opts, args)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.force, "force", "f", false, "Force removal of the image")
	flags.BoolVar(&opts.noPrune, "no-prune", false, "Do not delete untagged parents")

	return cmd
}

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := *NewRemoveCommand(dockerCli)
	cmd.Aliases = []string{"rmi", "remove"}
	cmd.Use = "rm [OPTIONS] IMAGE [IMAGE...]"
	return &cmd
}

func runRemove(dockerCli *command.DockerCli, opts removeOptions, images []string) error {
	client := dockerCli.Client()
	ctx := context.Background()

	options := types.ImageRemoveOptions{
		Force:         opts.force,
		PruneChildren: !opts.noPrune,
	}

	var errs []string
	for _, image := range images {
		dels, err := client.ImageRemove(ctx, image, options)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			for _, del := range dels {
				if del.Deleted != "" {
					fmt.Fprintf(dockerCli.Out(), "Deleted: %s\n", del.Deleted)
				} else {
					fmt.Fprintf(dockerCli.Out(), "Untagged: %s\n", del.Untagged)
				}
			}
		}
	}

	if len(errs) > 0 {
		return errors.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
