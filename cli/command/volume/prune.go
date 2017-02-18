package volume

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	units "github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type pruneOptions struct {
	force bool
}

// NewPruneCommand returns a new cobra prune command for volumes
func NewPruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts pruneOptions

	cmd := &cobra.Command{
		Use:   "prune [OPTIONS]",
		Short: "Remove all unused volumes",
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
		Tags: map[string]string{"version": "1.25"},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Do not prompt for confirmation")

	return cmd
}

const warning = `WARNING! This will remove all volumes not used by at least one container.
Are you sure you want to continue?`

func runPrune(dockerCli *command.DockerCli, opts pruneOptions) (spaceReclaimed uint64, output string, err error) {
	if !opts.force && !command.PromptForConfirmation(dockerCli.In(), dockerCli.Out(), warning) {
		return
	}

	report, err := dockerCli.Client().VolumesPrune(context.Background(), filters.Args{})
	if err != nil {
		return
	}

	if len(report.VolumesDeleted) > 0 {
		output = "Deleted Volumes:\n"
		for _, id := range report.VolumesDeleted {
			output += id + "\n"
		}
		spaceReclaimed = report.SpaceReclaimed
	}

	return
}

// RunPrune calls the Volume Prune API
// This returns the amount of space reclaimed and a detailed output string
func RunPrune(dockerCli *command.DockerCli) (uint64, string, error) {
	return runPrune(dockerCli, pruneOptions{force: true})
}
