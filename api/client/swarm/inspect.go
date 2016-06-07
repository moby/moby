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
	//	pretty  bool
}

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS]",
		Short: "Inspect the Swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// if opts.pretty && len(opts.format) > 0 {
			//	return fmt.Errorf("--format is incompatible with human friendly format")
			// }
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.Bool("help", false, "Print usage")
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	//flags.BoolVarP(&opts.pretty, "pretty", "h", false, "Print the information in a human friendly format.")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	getRef := func(_ string) (interface{}, []byte, error) {
		swarm, err := client.SwarmInspect(ctx)
		if err != nil {
			return nil, nil, err
		}
		return swarm, nil, nil
	}

	//	if !opts.pretty {
	return inspect.Inspect(dockerCli.Out(), []string{""}, opts.format, getRef)
	//	}

	//return printHumanFriendly(dockerCli.Out(), opts.refs, getRef)
}
