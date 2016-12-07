package system

import (
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/prune"
	"github.com/docker/docker/opts"
	units "github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type pruneOptions struct {
	force  bool
	all    bool
	filter opts.FilterOpt
}

// NewPruneCommand creates a new cobra.Command for `docker prune`
func NewPruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := pruneOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "prune [OPTIONS]",
		Short: "Remove unused data",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(dockerCli, opts)
		},
		Tags: map[string]string{"version": "1.25"},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Do not prompt for confirmation")
	flags.BoolVarP(&opts.all, "all", "a", false, "Remove all unused images not just dangling ones")
	flags.Var(&opts.filter, "filter", "Provide filter values (e.g. 'until=<timestamp>')")

	return cmd
}

const (
	warning = `WARNING! This will remove:
	- all stopped containers
	- all volumes not used by at least one container
	- all networks not used by at least one container
	%s
Are you sure you want to continue?`

	danglingImageDesc = "- all dangling images"
	allImageDesc      = `- all images without at least one container associated to them`
)

func runPrune(dockerCli *command.DockerCli, options pruneOptions) error {
	var message string

	if options.all {
		message = fmt.Sprintf(warning, allImageDesc)
	} else {
		message = fmt.Sprintf(warning, danglingImageDesc)
	}

	if !options.force && !command.PromptForConfirmation(dockerCli.In(), dockerCli.Out(), message) {
		return nil
	}

	var spaceReclaimed uint64

	for _, pruneFn := range []func(dockerCli *command.DockerCli, filter opts.FilterOpt) (uint64, string, error){
		prune.RunContainerPrune,
		prune.RunVolumePrune,
		prune.RunNetworkPrune,
	} {
		spc, output, err := pruneFn(dockerCli, options.filter)
		if err != nil {
			return err
		}
		spaceReclaimed += spc
		if output != "" {
			fmt.Fprintln(dockerCli.Out(), output)
		}
	}

	spc, output, err := prune.RunImagePrune(dockerCli, options.all, options.filter)
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
