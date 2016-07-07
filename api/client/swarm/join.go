package swarm

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type joinOptions struct {
	remote     string
	listenAddr NodeAddrOption
	manager    bool
	secret     string
	CACertHash string
}

func newJoinCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := joinOptions{
		listenAddr: NewListenAddrOption(),
	}

	cmd := &cobra.Command{
		Use:   "join [OPTIONS] HOST:PORT",
		Short: "Join a Swarm as a node and/or manager",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.remote = args[0]
			return runJoin(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, flagListenAddr, "Listen address")
	flags.BoolVar(&opts.manager, "manager", false, "Try joining as a manager.")
	flags.StringVar(&opts.secret, flagSecret, "", "Secret for node acceptance")
	flags.StringVar(&opts.CACertHash, "ca-hash", "", "Hash of the Root Certificate Authority certificate used for trusted join")
	return cmd
}

func runJoin(dockerCli *client.DockerCli, opts joinOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.JoinRequest{
		Manager:     opts.manager,
		Secret:      opts.secret,
		ListenAddr:  opts.listenAddr.String(),
		RemoteAddrs: []string{opts.remote},
		CACertHash:  opts.CACertHash,
	}
	err := client.SwarmJoin(ctx, req)
	if err != nil {
		return err
	}
	if opts.manager {
		fmt.Fprintln(dockerCli.Out(), "This node joined a Swarm as a manager.")
	} else {
		fmt.Fprintln(dockerCli.Out(), "This node joined a Swarm as a worker.")
	}
	return nil
}
