package swarm

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type initOptions struct {
	swarmOptions
	listenAddr NodeAddrOption
	// Not a NodeAddrOption because it has no default port.
	advertiseAddr   string
	forceNewCluster bool
	availability    string
}

func newInitCommand(dockerCli command.Cli) *cobra.Command {
	opts := initOptions{
		listenAddr: NewListenAddrOption(),
	}

	cmd := &cobra.Command{
		Use:   "init [OPTIONS]",
		Short: "Initialize a swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(dockerCli, cmd.Flags(), opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, flagListenAddr, "Listen address (format: <ip|interface>[:port])")
	flags.StringVar(&opts.advertiseAddr, flagAdvertiseAddr, "", "Advertised address (format: <ip|interface>[:port])")
	flags.BoolVar(&opts.forceNewCluster, "force-new-cluster", false, "Force create a new cluster from current state")
	flags.BoolVar(&opts.autolock, flagAutolock, false, "Enable manager autolocking (requiring an unlock key to start a stopped manager)")
	flags.StringVar(&opts.availability, flagAvailability, "active", "Availability of the node (active/pause/drain)")
	addSwarmFlags(flags, &opts.swarmOptions)
	return cmd
}

func runInit(dockerCli command.Cli, flags *pflag.FlagSet, opts initOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.InitRequest{
		ListenAddr:       opts.listenAddr.String(),
		AdvertiseAddr:    opts.advertiseAddr,
		ForceNewCluster:  opts.forceNewCluster,
		Spec:             opts.swarmOptions.ToSpec(flags),
		AutoLockManagers: opts.swarmOptions.autolock,
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

	nodeID, err := client.SwarmInit(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "could not choose an IP address to advertise") || strings.Contains(err.Error(), "could not find the system's IP address") {
			return errors.New(err.Error() + " - specify one with --advertise-addr")
		}
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "Swarm initialized: current node (%s) is now a manager.\n\n", nodeID)

	if err := printJoinCommand(ctx, dockerCli, nodeID, true, false); err != nil {
		return err
	}

	fmt.Fprint(dockerCli.Out(), "To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.\n\n")

	if req.AutoLockManagers {
		unlockKeyResp, err := client.SwarmGetUnlockKey(ctx)
		if err != nil {
			return errors.Wrap(err, "could not fetch unlock key")
		}
		printUnlockCommand(ctx, dockerCli, unlockKeyResp.UnlockKey)
	}

	return nil
}
