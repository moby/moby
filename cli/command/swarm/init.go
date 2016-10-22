package swarm

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type initOptions struct {
	swarmOptions
	listenAddr NodeAddrOption
	// Not a NodeAddrOption because it has no default port.
	advertiseAddr   string
	forceNewCluster bool
	lockKey         bool
}

func newInitCommand(dockerCli *command.DockerCli) *cobra.Command {
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
	flags.BoolVar(&opts.lockKey, flagLockKey, false, "Encrypt swarm with optionally provided key from stdin")
	flags.BoolVar(&opts.forceNewCluster, "force-new-cluster", false, "Force create a new cluster from current state")
	addSwarmFlags(flags, &opts.swarmOptions)
	return cmd
}

func runInit(dockerCli *command.DockerCli, flags *pflag.FlagSet, opts initOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	var lockKey string
	if opts.lockKey {
		var err error
		lockKey, err = readKey(dockerCli.In(), "Please enter key for encrypting swarm(leave empty to generate): ")
		if err != nil {
			return err
		}
		if len(lockKey) == 0 {
			randBytes := make([]byte, 16)
			if _, err := rand.Read(randBytes[:]); err != nil {
				panic(fmt.Errorf("failed to general random lock key: %v", err))
			}

			var n big.Int
			n.SetBytes(randBytes[:])
			lockKey = n.Text(36)
		}
	}

	req := swarm.InitRequest{
		ListenAddr:      opts.listenAddr.String(),
		AdvertiseAddr:   opts.advertiseAddr,
		ForceNewCluster: opts.forceNewCluster,
		Spec:            opts.swarmOptions.ToSpec(flags),
		LockKey:         lockKey,
	}

	nodeID, err := client.SwarmInit(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "could not choose an IP address to advertise") || strings.Contains(err.Error(), "could not find the system's IP address") {
			return errors.New(err.Error() + " - specify one with --advertise-addr")
		}
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "Swarm initialized: current node (%s) is now a manager.\n\n", nodeID)

	if len(lockKey) > 0 {
		fmt.Fprintf(dockerCli.Out(), "Swarm is encrypted. When a node is restarted it needs to be unlocked by running command:\n\n    echo '%s' | docker swarm unlock\n\n", lockKey)
	}

	if err := printJoinCommand(ctx, dockerCli, nodeID, true, false); err != nil {
		return err
	}

	fmt.Fprint(dockerCli.Out(), "To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.\n\n")
	return nil
}

func readKey(in *command.InStream, prompt string) (string, error) {
	if in.IsTerminal() {
		fmt.Print(prompt)
		dt, err := terminal.ReadPassword(int(in.FD()))
		fmt.Println()
		return string(dt), err
	} else {
		key, err := bufio.NewReader(in).ReadString('\n')
		if err == io.EOF {
			err = nil
		}
		return strings.TrimSpace(key), err
	}
}
