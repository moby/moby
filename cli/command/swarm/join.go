package swarm

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type joinOptions struct {
	remote     string
	listenAddr NodeAddrOption
	// Not a NodeAddrOption because it has no default port.
	advertiseAddr string
	token         string
}

func newJoinCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := joinOptions{
		listenAddr: NewListenAddrOption(),
	}

	cmd := &cobra.Command{
		Use:   "join [OPTIONS] HOST:PORT",
		Short: "Join a swarm as a node and/or manager",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.remote = args[0]
			return runJoin(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, flagListenAddr, "Listen address (format: <ip|interface>[:port])")
	flags.StringVar(&opts.advertiseAddr, flagAdvertiseAddr, "", "Advertised address (format: <ip|interface>[:port])")
	flags.StringVar(&opts.token, flagToken, "", "Token for entry into the swarm")
	return cmd
}

func runJoin(dockerCli *command.DockerCli, opts joinOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.JoinRequest{
		JoinToken:     opts.token,
		ListenAddr:    opts.listenAddr.String(),
		AdvertiseAddr: opts.advertiseAddr,
		RemoteAddrs:   []string{opts.remote},
	}
	err := client.SwarmJoin(ctx, req)
	if err != nil {
		return err
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	_, _, err = client.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	if err != nil {
		// TODO(aaronl): is there a better way to do this?
		if strings.Contains(err.Error(), "This node is not a swarm manager.") {
			fmt.Fprintln(dockerCli.Out(), "This node joined a swarm as a worker.")
		}
	} else {
		fmt.Fprintln(dockerCli.Out(), "This node joined a swarm as a manager.")
	}

	return nil
}
