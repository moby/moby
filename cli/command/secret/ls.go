package secret

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type listOptions struct {
	quiet  bool
	format string
	filter opts.FilterOpt
}

func newSecretListCommand(dockerCli command.Cli) *cobra.Command {
	opts := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List secrets",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display IDs")
	flags.StringVarP(&opts.format, "format", "", "", "Pretty-print secrets using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runSecretList(dockerCli command.Cli, opts listOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	secrets, err := client.SecretList(ctx, types.SecretListOptions{Filters: opts.filter.Value()})
	if err != nil {
		return err
	}
	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().SecretFormat) > 0 && !opts.quiet {
			format = dockerCli.ConfigFile().SecretFormat
		} else {
			format = formatter.TableFormatKey
		}
	}
	secretCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewSecretFormat(format, opts.quiet),
	}
	return formatter.SecretWrite(secretCtx, secrets)
}
