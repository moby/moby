package swarm

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type joinOptions struct {
	remote     string
	listenAddr NodeAddrOption
	// Not a NodeAddrOption because it has no default port.
	advertiseAddr string
	token         string
	availability  string
}

func newJoinCommand(dockerCli command.Cli) *cobra.Command {
	opts := joinOptions{
		listenAddr: NewListenAddrOption(),
	}

	cmd := &cobra.Command{
		Use:   "join [OPTIONS] HOST:PORT",
		Short: "Join a swarm as a node and/or manager",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.remote = args[0]
			return runJoin(dockerCli, cmd.Flags(), opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, flagListenAddr, "Listen address (format: <ip|interface>[:port])")
	flags.StringVar(&opts.advertiseAddr, flagAdvertiseAddr, "", "Advertised address (format: <ip|interface>[:port])")
	flags.StringVar(&opts.token, flagToken, "", "Token for entry into the swarm")
	flags.StringVar(&opts.availability, flagAvailability, "active", "Availability of the node (active/pause/drain)")
	return cmd
}

func runJoin(dockerCli command.Cli, flags *pflag.FlagSet, opts joinOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.JoinRequest{
		JoinToken:     opts.token,
		ListenAddr:    opts.listenAddr.String(),
		AdvertiseAddr: opts.advertiseAddr,
		RemoteAddrs:   []string{opts.remote},
	}
	if flags.Changed(flagAvailability) {
		availability := swarm.NodeAvailability(strings.ToLower(opts.availability))
		switch availability {
		case swarm.NodeAvailabilityActive, swarm.NodeAvailabilityPause, swarm.NodeAvailabilityDrain:
			req.Availability = availability
		default:
			return fmt.Errorf("invalid availability %q, only active, pause and drain are supported", opts.availability)
		}
	}

	err := client.SwarmJoin(ctx, req)
	if err != nil {
		return err
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	if info.Swarm.ControlAvailable {
		fmt.Fprintln(dockerCli.Out(), "This node joined a swarm as a manager.")
	} else {
		fmt.Fprintln(dockerCli.Out(), "This node joined a swarm as a worker.")
	}
	return nil
}
