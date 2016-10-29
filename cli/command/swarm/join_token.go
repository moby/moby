package swarm

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"golang.org/x/net/context"
)

func newJoinTokenCommand(dockerCli *command.DockerCli) *cobra.Command {
	var rotate, quiet bool

	cmd := &cobra.Command{
		Use:   "join-token [OPTIONS] (worker|manager)",
		Short: "Manage join tokens",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			worker := args[0] == "worker"
			manager := args[0] == "manager"

			if !worker && !manager {
				return errors.New("unknown role " + args[0])
			}

			client := dockerCli.Client()
			ctx := context.Background()

			if rotate {
				var flags swarm.UpdateFlags

				swarm, err := client.SwarmInspect(ctx)
				if err != nil {
					return err
				}

				flags.RotateWorkerToken = worker
				flags.RotateManagerToken = manager

				err = client.SwarmUpdate(ctx, swarm.Version, swarm.Spec, flags)
				if err != nil {
					return err
				}
				if !quiet {
					fmt.Fprintf(dockerCli.Out(), "Successfully rotated %s join token.\n\n", args[0])
				}
			}

			swarm, err := client.SwarmInspect(ctx)
			if err != nil {
				return err
			}

			if quiet {
				if worker {
					fmt.Fprintln(dockerCli.Out(), swarm.JoinTokens.Worker)
				} else {
					fmt.Fprintln(dockerCli.Out(), swarm.JoinTokens.Manager)
				}
			} else {
				info, err := client.Info(ctx)
				if err != nil {
					return err
				}
				return printJoinCommand(ctx, dockerCli, info.Swarm.NodeID, worker, manager)
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&rotate, flagRotate, false, "Rotate join token")
	flags.BoolVarP(&quiet, flagQuiet, "q", false, "Only display token")

	return cmd
}

func printJoinCommand(ctx context.Context, dockerCli *command.DockerCli, nodeID string, worker bool, manager bool) error {
	client := dockerCli.Client()

	swarm, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	node, _, err := client.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return err
	}

	if node.ManagerStatus != nil {
		if worker {
			fmt.Fprintf(dockerCli.Out(), "To add a worker to this swarm, run the following command:\n\n    docker swarm join \\\n    --token %s \\\n    %s\n\n", swarm.JoinTokens.Worker, node.ManagerStatus.Addr)
		}
		if manager {
			fmt.Fprintf(dockerCli.Out(), "To add a manager to this swarm, run the following command:\n\n    docker swarm join \\\n    --token %s \\\n    %s\n\n", swarm.JoinTokens.Manager, node.ManagerStatus.Addr)
		}
	}

	return nil
}
