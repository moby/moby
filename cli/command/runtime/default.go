package runtime

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newDefaultCommand(dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "default RUNTIME [VOLUME...]",
		Aliases: []string{"default"},
		Short:   "Select a default runtime",
		Long:    defaultDescription,
		Example: defaultExample,
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDefault(dockerCli, args[0])
		},
	}

	return cmd
}

func runDefault(dockerCli command.Cli, runtime string) error {
	client := dockerCli.Client()

	return client.RuntimeDefault(context.Background(), runtime)
}

var defaultDescription = `
Set the default runtime. This overrides the default runtime selection done through the dockerd command line.
`

var defaultExample = `
$ docker runtime default clearcontainers/runtime:2.1.3
clearcontainers/runtime:2.1.3
`
