package swarm

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format string
}

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS]",
		Short: "Inspect the swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	swarm, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	getRef := func(_ string) (interface{}, []byte, error) {
		return swarm, nil, nil
	}

	return inspect.Inspect(dockerCli.Out(), []string{""}, opts.format, getRef)
}
