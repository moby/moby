package network

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

func newRemoveCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NETWORK [NETWORK]...",
		Aliases: []string{"remove"},
		Short:   "Remove a network",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args)
		},
	}
}

func runRemove(dockerCli *client.DockerCli, networks []string) error {
	client := dockerCli.Client()
	ctx := context.Background()
	status := 0

	for _, name := range networks {
		if err := client.NetworkRemove(ctx, name); err != nil {
			fmt.Fprintf(dockerCli.Err(), "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(dockerCli.Err(), "%s\n", name)
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}
	return nil
}
