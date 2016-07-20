// +build experimental

package stack

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	namespace string
}

func newRemoveCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "rm STACK",
		Aliases: []string{"remove", "down"},
		Short:   "Remove the stack",
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.namespace = args[0]
			return runRemove(dockerCli, opts)
		},
	}
	return cmd
}

func runRemove(dockerCli *client.DockerCli, opts removeOptions) error {
	namespace := opts.namespace
	client := dockerCli.Client()

	stderr := dockerCli.Err()
	stdout := dockerCli.Out()

	ctx := context.Background()
	hasError := false

	services, err := getServices(ctx, client, namespace)
	if err != nil {
		return err
	}
	for _, service := range services {
		fmt.Fprintf(stdout, "Removing service %s\n", service.Spec.Name)
		if err := client.ServiceRemove(ctx, service.ID); err != nil {
			hasError = true
			fmt.Fprintf(stderr, "Failed to remove service %s: %s", service.ID, err)
		} else {
			fmt.Fprintln(stdout, "Succeed")
		}
	}

	networks, err := getNetworks(ctx, client, namespace)
	if err != nil {
		return err
	}
	for _, network := range networks {
		fmt.Fprintf(stdout, "Removing network %s\n", network.Name)
		if err := client.NetworkRemove(ctx, network.ID); err != nil {
			hasError = true
			fmt.Fprintf(stderr, "Failed to remove network %s: %s", network.ID, err)
		} else {
			fmt.Fprintln(stdout, "Succeed")
		}
	}

	if len(services) == 0 && len(networks) == 0 {
		fmt.Fprintf(stdout, "Nothing found in stack: %s\n", namespace)
		return nil
	}

	if hasError {
		return fmt.Errorf("Failed to remove some resources")
	}
	return nil
}
