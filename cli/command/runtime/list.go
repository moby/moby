package runtime

import (
	"sort"

	"github.com/docker/docker/api/types/runtime"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type byRuntimeName []runtime.Info

func (r byRuntimeName) Len() int      { return len(r) }
func (r byRuntimeName) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byRuntimeName) Less(i, j int) bool {
	return r[i].Name < r[j].Name
}

type listOptions struct {
	quiet  bool
	format string
	filter opts.FilterOpt
}

func newListCommand(dockerCli command.Cli) *cobra.Command {
	opts := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List runtimes",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display runtime names")
	flags.StringVar(&opts.format, "format", "", "Pretty-print runtimes using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Provide filter values (e.g. 'dangling=true')")

	return cmd
}

func runList(dockerCli command.Cli, opts listOptions) error {
	client := dockerCli.Client()
	runtimes, err := client.RuntimeList(context.Background(), opts.filter.Value())
	if err != nil {
		return err
	}

	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().RuntimesFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().RuntimesFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	sort.Sort(byRuntimeName(runtimes))

	runtimeCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewRuntimeFormat(format, opts.quiet),
	}

	return formatter.RuntimeWrite(runtimeCtx, runtimes)
}
