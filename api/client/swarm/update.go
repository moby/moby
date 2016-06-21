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

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := swarmOptions{autoAccept: NewAutoAcceptOption()}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the Swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), opts)
		},
	}

	addSwarmFlags(cmd.Flags(), &opts)
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, flags *pflag.FlagSet, opts swarmOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	swarm, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	err = mergeSwarm(&swarm, flags)
	if err != nil {
		return err
	}

	err = client.SwarmUpdate(ctx, swarm.Version, swarm.Spec)
	if err != nil {
		return err
	}

	fmt.Println("Swarm updated.")
	return nil
}

func mergeSwarm(swarm *swarm.Swarm, flags *pflag.FlagSet) error {
	spec := &swarm.Spec

	if flags.Changed(flagAutoAccept) {
		value := flags.Lookup(flagAutoAccept).Value.(*AutoAcceptOption)
		spec.AcceptancePolicy.Policies = value.Policies(nil)
	}

	var psecret *string
	if flags.Changed(flagSecret) {
		secret, _ := flags.GetString(flagSecret)
		psecret = &secret
	}

	for i := range spec.AcceptancePolicy.Policies {
		spec.AcceptancePolicy.Policies[i].Secret = psecret
	}

	if flags.Changed(flagTaskHistoryLimit) {
		spec.Orchestration.TaskHistoryRetentionLimit, _ = flags.GetInt64(flagTaskHistoryLimit)
	}

	if flags.Changed(flagDispatcherHeartbeat) {
		if v, err := flags.GetDuration(flagDispatcherHeartbeat); err == nil {
			spec.Dispatcher.HeartbeatPeriod = uint64(v.Nanoseconds())
		}
	}

	if flags.Changed(flagCertExpiry) {
		if v, err := flags.GetDuration(flagCertExpiry); err == nil {
			spec.CAConfig.NodeCertExpiry = v
		}
	}

	return nil
}
