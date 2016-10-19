package secret

import (
	"context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	name   string
	format string
}

func newSecretInspectCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := inspectOptions{}
	cmd := &cobra.Command{
		Use:   "inspect [name]",
		Short: "Inspect a secret",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			return runSecretInspect(dockerCli, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	return cmd
}

func runSecretInspect(dockerCli *command.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	getRef := func(name string) (interface{}, []byte, error) {
		return client.SecretInspectWithRaw(ctx, name)
	}

	return inspect.Inspect(dockerCli.Out(), []string{opts.name}, opts.format, getRef)
}
