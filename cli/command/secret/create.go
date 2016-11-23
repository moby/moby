package secret

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type createOptions struct {
	name   string
	labels opts.ListOpts
}

func newSecretCreateCommand(dockerCli *command.DockerCli) *cobra.Command {
	createOpts := createOptions{
		labels: opts.NewListOpts(runconfigopts.ValidateEnv),
	}

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] SECRET",
		Short: "Create a secret using stdin as content",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			createOpts.name = args[0]
			return runSecretCreate(dockerCli, createOpts)
		},
	}
	flags := cmd.Flags()
	flags.VarP(&createOpts.labels, "label", "l", "Secret labels")

	return cmd
}

func runSecretCreate(dockerCli *command.DockerCli, options createOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	secretData, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("Error reading content from STDIN: %v", err)
	}

	spec := swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name:   options.name,
			Labels: runconfigopts.ConvertKVStringsToMap(options.labels.GetAll()),
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
