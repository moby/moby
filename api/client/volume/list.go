package volume

import (
	"sort"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/formatter"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
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
	filter []string
}

func newListCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List volumes",
		Long:    listDescription,
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display volume names")
	flags.StringVar(&opts.format, "format", "", "Pretty-print networks using a Go template")
	flags.StringSliceVarP(&opts.filter, "filter", "f", []string{}, "Provide filter values (e.g. 'dangling=true')")

	return cmd
}

func runList(dockerCli *client.DockerCli, opts listOptions) error {
	client := dockerCli.Client()

	volFilterArgs := filters.NewArgs()
	for _, f := range opts.filter {
		var err error
		volFilterArgs, err = filters.ParseFlag(f, volFilterArgs)
		if err != nil {
			return err
		}
	}

	volumes, err := client.VolumeList(context.Background(), volFilterArgs)
	if err != nil {
		return err
	}

	f := opts.format
	if len(f) == 0 {
		if len(dockerCli.ConfigFile().VolumesFormat) > 0 && !opts.quiet {
			f = dockerCli.ConfigFile().VolumesFormat
		} else {
			f = "table"
		}
	}

	sort.Sort(byVolumeName(volumes.Volumes))

	volumeCtx := formatter.VolumeContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: f,
			Quiet:  opts.quiet,
		},
		Volumes: volumes.Volumes,
	}

	volumeCtx.Write()

	return nil
}

var listDescription = `

Lists all the volumes Docker knows about. You can filter using the **-f** or
**--filter** flag. The filtering format is a **key=value** pair. To specify
more than one filter,  pass multiple flags (for example,
**--filter "foo=bar" --filter "bif=baz"**)

The currently supported filters are:

* **dangling** (boolean - **true** or **false**, **1** or **0**)
* **driver** (a volume driver's name)
* **label** (**label=<key>** or **label=<key>=<value>**)
* **name** (a volume's name)

`
