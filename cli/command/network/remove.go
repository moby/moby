package network

import (
	"fmt"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type removeOptions struct {
	force bool

	networks []string
}

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "rm NETWORK [NETWORK...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more networks",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.networks = args
			return runRemove(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Force the removal of one or more networks")
	flags.SetAnnotation("force", "version", []string{"1.28"})

	return cmd
}

func runRemove(dockerCli *command.DockerCli, opts *removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	status := 0

	for _, name := range opts.networks {
		if err := client.NetworkRemove(ctx, name, opts.force); err != nil {
			fmt.Fprintf(dockerCli.Err(), "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", name)
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}
	return nil
}
