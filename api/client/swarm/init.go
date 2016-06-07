package swarm

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

type initOptions struct {
	listenAddr      NodeAddrOption
	autoAccept      AutoAcceptOption
	forceNewCluster bool
	secret          string
}

func newInitCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := initOptions{
		listenAddr: NewNodeAddrOption(),
		autoAccept: NewAutoAcceptOption(),
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a Swarm.",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, "listen-addr", "Listen address")
	flags.Var(&opts.autoAccept, "auto-accept", "Acceptance policy")
	flags.StringVar(&opts.secret, "secret", "", "Set secret value needed to accept nodes into cluster")
	flags.BoolVar(&opts.forceNewCluster, "force-new-cluster", false, "Force create a new cluster from current state.")
	return cmd
}

func runInit(dockerCli *client.DockerCli, opts initOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.InitRequest{
		ListenAddr:      opts.listenAddr.String(),
		ForceNewCluster: opts.forceNewCluster,
	}

	req.Spec.AcceptancePolicy.Policies = opts.autoAccept.Policies(opts.secret)

	nodeID, err := client.SwarmInit(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("Swarm initialized: current node (%s) is now a manager.\n", nodeID)
	return nil
}
