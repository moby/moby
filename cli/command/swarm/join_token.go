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

type joinOption struct {
	quiet  bool
	rotate bool
}

func newJoinTokenCommand(dockerCli command.Cli) *cobra.Command {
	var opt joinOption

	cmd := &cobra.Command{
		Use:   "join-token [OPTIONS] (worker|manager)",
		Short: "Manage join tokens",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJoinToken(dockerCli, opt, args)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opt.rotate, flagRotate, false, "Rotate join token")
	flags.BoolVarP(&opt.quiet, flagQuiet, "q", false, "Only display token")

	return cmd
}

func runJoinToken(dockerCli command.Cli, opt joinOption, args []string) error {
	worker := args[0] == "worker"
	manager := args[0] == "manager"

	if !worker && !manager {
		return errors.New("unknown role " + args[0])
	}

	client := dockerCli.Client()
	ctx := context.Background()

	if opt.rotate {
		swarmInspect, err := client.SwarmInspect(ctx)
		if err != nil {
			return err
		}

		var flags swarm.UpdateFlags

		flags.RotateWorkerToken = worker
		flags.RotateManagerToken = manager

		err = client.SwarmUpdate(ctx, swarmInspect.Version, swarmInspect.Spec, flags)
		if err != nil {
			return err
		}
		if !opt.quiet {
			fmt.Fprintf(dockerCli.Out(), "Successfully rotated %s join token.\n\n", args[0])
		}
	}

	var nodeID string
	if !opt.quiet {
		info, err := client.Info(ctx)
		if err != nil {
			return err
		}
		nodeID = info.Swarm.NodeID
	}
	return printJoinCommand(ctx, dockerCli, nodeID, opt.quiet, worker, manager)
}

func printJoinCommand(ctx context.Context, dockerCli command.Cli, nodeID string, quiet, worker, manager bool) error {
	client := dockerCli.Client()

	swarmInspect, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	if quiet {
		if worker {
			fmt.Fprintln(dockerCli.Out(), swarmInspect.JoinTokens.Worker)
		} else {
			fmt.Fprintln(dockerCli.Out(), swarmInspect.JoinTokens.Manager)
		}
		return nil
	}

	node, _, err := client.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return err
	}

	if node.ManagerStatus != nil {
		if worker {
			fmt.Fprintf(dockerCli.Out(), "To add a worker to this swarm, run the following command:\n\n    docker swarm join \\\n    --token %s \\\n    %s\n\n", swarmInspect.JoinTokens.Worker, node.ManagerStatus.Addr)
		}
		if manager {
			fmt.Fprintf(dockerCli.Out(), "To add a manager to this swarm, run the following command:\n\n    docker swarm join \\\n    --token %s \\\n    %s\n\n", swarmInspect.JoinTokens.Manager, node.ManagerStatus.Addr)
		}
	}

	return nil
}
