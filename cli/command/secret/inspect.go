package secret

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
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

	// attempt to lookup secret by name
	secrets, err := getSecretsByName(ctx, client, []string{opts.name})
	if err != nil {
		return err
	}

	id := opts.name
	for _, s := range secrets {
		if s.Spec.Annotations.Name == opts.name {
			id = s.ID
			break
		}
	}

	getRef := func(name string) (interface{}, []byte, error) {
		return client.SecretInspectWithRaw(ctx, id)
	}

	return inspect.Inspect(dockerCli.Out(), []string{id}, opts.format, getRef)
}
