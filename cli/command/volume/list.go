package volume

import (
	"sort"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

type byVolumeName []*types.Volume

func (r byVolumeName) Len() int      { return len(r) }
func (r byVolumeName) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byVolumeName) Less(i, j int) bool {
	return r[i].Name < r[j].Name
}

type listOptions struct {
	quiet  bool
	format string
	filter opts.FilterOpt
}

func newListCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List volumes",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display volume names")
	flags.StringVar(&opts.format, "format", "", "Pretty-print volumes using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Provide filter values (e.g. 'dangling=true')")

	return cmd
}

func runList(dockerCli *command.DockerCli, opts listOptions) error {
	client := dockerCli.Client()
	volumes, err := client.VolumeList(context.Background(), opts.filter.Value())
	if err != nil {
		return err
	}

	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().VolumesFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().VolumesFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	sort.Sort(byVolumeName(volumes.Volumes))

	volumeCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewVolumeFormat(format, opts.quiet),
	}
	return formatter.VolumeWrite(volumeCtx, volumes.Volumes)
}
