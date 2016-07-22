package swarm

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	generatedSecretEntropyBytes = 16
	generatedSecretBase         = 36
	// floor(log(2^128-1, 36)) + 1
	maxGeneratedSecretLength = 25
)

type initOptions struct {
	swarmOptions
	listenAddr      NodeAddrOption
	forceNewCluster bool
}

func newInitCommand(dockerCli *client.DockerCli) *cobra.Command {
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
	flags.Var(&opts.listenAddr, "listen-addr", "Listen address")
	flags.BoolVar(&opts.forceNewCluster, "force-new-cluster", false, "Force create a new cluster from current state.")
	addSwarmFlags(flags, &opts.swarmOptions)
	return cmd
}

func runInit(dockerCli *client.DockerCli, flags *pflag.FlagSet, opts initOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.InitRequest{
		ListenAddr:      opts.listenAddr.String(),
		ForceNewCluster: opts.forceNewCluster,
		Spec:            opts.swarmOptions.ToSpec(),
	}

	nodeID, err := client.SwarmInit(ctx, req)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "Swarm initialized: current node (%s) is now a manager.\n\n", nodeID)

	return printJoinCommand(ctx, dockerCli, nodeID, true, true)
}
