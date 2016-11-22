package secret

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type inspectOptions struct {
	names  []string
	format string
}

func newSecretInspectCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := inspectOptions{}
	cmd := &cobra.Command{
		Use:   "inspect SECRET [SECRET]",
		Short: "Inspect a secret",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.names = args
			return runSecretInspect(dockerCli, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	return cmd
}

func runSecretInspect(dockerCli *command.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	ids, err := getCliRequestedSecretIDs(ctx, client, opts.names)
	if err != nil {
		return err
	}
	getRef := func(id string) (interface{}, []byte, error) {
		return client.SecretInspectWithRaw(ctx, id)
	}

	return inspect.Inspect(dockerCli.Out(), ids, opts.format, getRef)
}
