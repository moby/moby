package network

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NETWORK [NETWORK...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more networks",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args)
		},
	}
}

const ingressWarning = "WARNING! Before removing the routing-mesh network, " +
	"make sure all the nodes in your swarm run the same docker engine version. " +
	"Otherwise, removal may not be effective and functionality of newly create " +
	"ingress networks will be impaired.\nAre you sure you want to continue?"

func runRemove(dockerCli *command.DockerCli, networks []string) error {
	client := dockerCli.Client()
	ctx := context.Background()
	status := 0

	for _, name := range networks {
		if nw, _, err := client.NetworkInspectWithRaw(ctx, name, false); err == nil &&
			nw.Ingress &&
			!command.PromptForConfirmation(dockerCli.In(), dockerCli.Out(), ingressWarning) {
			continue
		}
		if err := client.NetworkRemove(ctx, name); err != nil {
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
