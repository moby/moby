package container

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/formatter"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"

	"github.com/docker/docker/utils/templates"
	"github.com/spf13/cobra"
	"io/ioutil"
)

type psOptions struct {
	quiet   bool
	size    bool
	all     bool
	noTrunc bool
	nLatest bool
	last    int
	format  string
	filter  []string
}

type preProcessor struct {
	opts *types.ContainerListOptions
}

// Size sets the size option when called by a template execution.
func (p *preProcessor) Size() bool {
	p.opts.Size = true
	return true
}

// NewPsCommand creates a new cobra.Command for `docker ps`
func NewPsCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts psOptions

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS]",
		Short: "List containers",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPs(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display numeric IDs")
	flags.BoolVarP(&opts.size, "size", "s", false, "Display total file sizes")
	flags.BoolVarP(&opts.all, "all", "a", false, "Show all containers (default shows just running)")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Don't truncate output")
	flags.BoolVarP(&opts.nLatest, "latest", "l", false, "Show the latest created container (includes all states)")
	flags.IntVarP(&opts.last, "last", "n", -1, "Show n last created containers (includes all states)")
	flags.StringVarP(&opts.format, "format", "", "", "Pretty-print containers using a Go template")
	flags.StringSliceVarP(&opts.filter, "filter", "f", []string{}, "Filter output based on conditions provided")

	return cmd
}

func runPs(dockerCli *client.DockerCli, opts *psOptions) error {
	ctx := context.Background()

	if opts.nLatest && opts.last == -1 {
		opts.last = 1
	}

	containerFilterArgs := filters.NewArgs()
	for _, f := range opts.filter {
		var err error
		containerFilterArgs, err = filters.ParseFlag(f, containerFilterArgs)
		if err != nil {
			return err
		}
	}

	options := types.ContainerListOptions{
		All:    opts.all,
		Limit:  opts.last,
		Size:   opts.size,
		Filter: containerFilterArgs,
	}

	pre := &preProcessor{opts: &options}
	tmpl, err := templates.Parse(opts.format)

	if err != nil {
		return err
	}

	_ = tmpl.Execute(ioutil.Discard, pre)

	containers, err := dockerCli.Client().ContainerList(ctx, options)
	if err != nil {
		return err
	}

	f := opts.format
	if len(f) == 0 {
		if len(dockerCli.PsFormat()) > 0 && !opts.quiet {
			f = dockerCli.PsFormat()
		} else {
			f = "table"
		}
	}

	psCtx := formatter.ContainerContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: f,
			Quiet:  opts.quiet,
			Trunc:  !opts.noTrunc,
		},
		Size:       opts.size,
		Containers: containers,
	}

	psCtx.Write()

	return nil
}
