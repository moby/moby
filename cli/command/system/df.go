package system

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type diskUsageOptions struct {
	verbose bool
}

// NewDiskUsageCommand creates a new cobra.Command for `docker df`
func NewDiskUsageCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts diskUsageOptions

	cmd := &cobra.Command{
		Use:   "df [OPTIONS]",
		Short: "Show docker disk usage",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiskUsage(dockerCli, opts)
		},
		Tags: map[string]string{"version": "1.25"},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.verbose, "verbose", "v", false, "Show detailed information on space usage")

	return cmd
}

func runDiskUsage(dockerCli *command.DockerCli, opts diskUsageOptions) error {
	du, err := dockerCli.Client().DiskUsage(context.Background())
	if err != nil {
		return err
	}

	duCtx := formatter.DiskUsageContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
		},
		LayersSize: du.LayersSize,
		Images:     du.Images,
		Containers: du.Containers,
		Volumes:    du.Volumes,
		Verbose:    opts.verbose,
	}

	duCtx.Write()

	return nil
}
