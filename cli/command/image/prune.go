package image

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	units "github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type pruneOptions struct {
	force bool
	all   bool
}

// NewPruneCommand returns a new cobra prune command for images
func NewPruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts pruneOptions

	cmd := &cobra.Command{
		Use:   "prune [OPTIONS]",
		Short: "Remove unused images",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			spaceReclaimed, output, err := runPrune(dockerCli, opts)
			if err != nil {
				return err
			}
			if output != "" {
				fmt.Fprintln(dockerCli.Out(), output)
			}
			fmt.Fprintln(dockerCli.Out(), "Total reclaimed space:", units.HumanSize(float64(spaceReclaimed)))
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Do not prompt for confirmation")
	flags.BoolVarP(&opts.all, "all", "a", false, "Remove all unused images, not just dangling ones")

	return cmd
}

const (
	allImageWarning = `WARNING! This will remove all images without at least one container associated to them.
Are you sure you want to continue?`
	danglingWarning = `WARNING! This will remove all dangling images.
Are you sure you want to continue?`
)

func runPrune(dockerCli *command.DockerCli, opts pruneOptions) (spaceReclaimed uint64, output string, err error) {
	warning := danglingWarning
	if opts.all {
		warning = allImageWarning
	}
	if !opts.force && !command.PromptForConfirmation(dockerCli.In(), dockerCli.Out(), warning) {
		return
	}

	report, err := dockerCli.Client().ImagesPrune(context.Background(), types.ImagesPruneConfig{
		DanglingOnly: !opts.all,
	})
	if err != nil {
		return
	}

	if len(report.ImagesDeleted) > 0 {
		output = "Deleted Images:\n"
		for _, st := range report.ImagesDeleted {
			if st.Untagged != "" {
				output += fmt.Sprintln("untagged:", st.Untagged)
			} else {
				output += fmt.Sprintln("deleted:", st.Deleted)
			}
		}
		spaceReclaimed = report.SpaceReclaimed
	}

	return
}

// RunPrune calls the Image Prune API
// This returns the amount of space reclaimed and a detailed output string
func RunPrune(dockerCli *command.DockerCli, all bool) (uint64, string, error) {
	return runPrune(dockerCli, pruneOptions{force: true, all: all})
}
