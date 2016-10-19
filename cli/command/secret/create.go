package secret

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type createOptions struct {
	name string
}

func newSecretCreateCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "create [name]",
		Short: "Create a secret using stdin as content",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := createOptions{
				name: args[0],
			}

			return runSecretCreate(dockerCli, opts)
		},
	}
}

func runSecretCreate(dockerCli *command.DockerCli, opts createOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	secretData, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("Error reading content from STDIN: %v", err)
	}

	spec := swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: opts.name,
		},
		Data: secretData,
	}

	r, err := client.SecretCreate(ctx, spec)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), r.ID)
	return nil
}
