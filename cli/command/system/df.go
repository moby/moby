package system

import (
	"errors"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type diskUsageOptions struct {
	verbose bool
	format  string
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
	flags.StringVar(&opts.format, "format", "", "Pretty-print images using a Go template")

	return cmd
}

func runDiskUsage(dockerCli *command.DockerCli, opts diskUsageOptions) error {
	if opts.verbose && len(opts.format) != 0 {
		return errors.New("the verbose and the format options conflict")
	}

	du, err := dockerCli.Client().DiskUsage(context.Background())
	if err != nil {
		return err
	}

	format := opts.format
	if len(format) == 0 {
		format = formatter.TableFormatKey
	}

	duCtx := formatter.DiskUsageContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: formatter.NewDiskUsageFormat(format),
		},
		LayersSize: du.LayersSize,
		Images:     du.Images,
		Containers: du.Containers,
		Volumes:    du.Volumes,
		Verbose:    opts.verbose,
	}

	return duCtx.Write()
}
