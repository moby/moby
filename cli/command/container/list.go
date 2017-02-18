package container

import (
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/templates"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type psOptions struct {
	quiet   bool
	size    bool
	all     bool
	noTrunc bool
	nLatest bool
	last    int
	format  string
	filter  opts.FilterOpt
}

// NewPsCommand creates a new cobra.Command for `docker ps`
func NewPsCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

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
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func newListCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := *NewPsCommand(dockerCli)
	cmd.Aliases = []string{"ps", "list"}
	cmd.Use = "ls [OPTIONS]"
	return &cmd
}

// listOptionsProcessor is used to set any container list options which may only
// be embedded in the format template.
// This is passed directly into tmpl.Execute in order to allow the preprocessor
// to set any list options that were not provided by flags (e.g. `.Size`).
// It is using a `map[string]bool` so that unknown fields passed into the
// template format do not cause errors. These errors will get picked up when
// running through the actual template processor.
type listOptionsProcessor map[string]bool

// Size sets the size of the map when called by a template execution.
func (o listOptionsProcessor) Size() bool {
	o["size"] = true
	return true
}

// Label is needed here as it allows the correct pre-processing
// because Label() is a method with arguments
func (o listOptionsProcessor) Label(name string) string {
	return ""
}

func buildContainerListOptions(opts *psOptions) (*types.ContainerListOptions, error) {
	options := &types.ContainerListOptions{
		All:     opts.all,
		Limit:   opts.last,
		Size:    opts.size,
		Filters: opts.filter.Value(),
	}

	if opts.nLatest && opts.last == -1 {
		options.Limit = 1
	}

	tmpl, err := templates.Parse(opts.format)

	if err != nil {
		return nil, err
	}

	optionsProcessor := listOptionsProcessor{}
	// This shouldn't error out but swallowing the error makes it harder
	// to track down if preProcessor issues come up. Ref #24696
	if err := tmpl.Execute(ioutil.Discard, optionsProcessor); err != nil {
		return nil, err
	}
	// At the moment all we need is to capture .Size for preprocessor
	options.Size = opts.size || optionsProcessor["size"]

	return options, nil
}

func runPs(dockerCli *command.DockerCli, opts *psOptions) error {
	ctx := context.Background()

	listOptions, err := buildContainerListOptions(opts)
	if err != nil {
		return err
	}

	containers, err := dockerCli.Client().ContainerList(ctx, *listOptions)
	if err != nil {
		return err
	}

	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().PsFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().PsFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	containerCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewContainerFormat(format, opts.quiet, listOptions.Size),
		Trunc:  !opts.noTrunc,
	}
	return formatter.ContainerWrite(containerCtx, containers)
}
