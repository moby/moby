package system

import (
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/prune"
	units "github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type pruneOptions struct {
	force bool
	all   bool
}

// NewPruneCommand creates a new cobra.Command for `docker prune`
func NewPruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts pruneOptions

	cmd := &cobra.Command{
		Use:   "prune [OPTIONS]",
		Short: "Remove unused data",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Do not prompt for confirmation")
	flags.BoolVarP(&opts.all, "all", "a", false, "Remove all unused images not just dangling ones")

	return cmd
}

const (
	warning = `WARNING! This will remove:
	- all stopped containers
	- all volumes not used by at least one container
	%s
Are you sure you want to continue?`

	danglingImageDesc = "- all dangling images"
	allImageDesc      = `- all images without at least one container associated to them`
)

func runPrune(dockerCli *command.DockerCli, opts pruneOptions) error {
	var message string

	if opts.all {
		message = fmt.Sprintf(warning, allImageDesc)
	} else {
		message = fmt.Sprintf(warning, danglingImageDesc)
	}

	if !opts.force && !command.PromptForConfirmation(dockerCli.In(), dockerCli.Out(), message) {
		return nil
	}

	var spaceReclaimed uint64

	for _, pruneFn := range []func(dockerCli *command.DockerCli) (uint64, string, error){
		prune.RunContainerPrune,
		prune.RunVolumePrune,
	} {
		spc, output, err := pruneFn(dockerCli)
		if err != nil {
			return err
		}
		if spc > 0 {
			spaceReclaimed += spc
			fmt.Fprintln(dockerCli.Out(), output)
		}
	}

	spc, output, err := prune.RunImagePrune(dockerCli, opts.all)
	if err != nil {
		return err
	}
	if spc > 0 {
		spaceReclaimed += spc
		fmt.Fprintln(dockerCli.Out(), output)
	}

	fmt.Fprintln(dockerCli.Out(), "Total reclaimed space:", units.HumanSize(float64(spaceReclaimed)))

	return nil
}
