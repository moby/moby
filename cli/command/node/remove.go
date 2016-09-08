package node

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	force bool
}

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := removeOptions{}

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] NODE [NODE...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more nodes from the swarm",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.force, "force", false, "Force remove an active node")
	return cmd
}

func runRemove(dockerCli *command.DockerCli, args []string, opts removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	for _, nodeID := range args {
		err := client.NodeRemove(ctx, nodeID, types.NodeRemoveOptions{Force: opts.force})
		if err != nil {
			return err
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", nodeID)
	}
	return nil
}
