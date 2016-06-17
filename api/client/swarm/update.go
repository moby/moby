package swarm

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type updateOptions struct {
	autoAccept          AutoAcceptOption
	secret              string
	taskHistoryLimit    int64
	dispatcherHeartbeat time.Duration
	nodeCertExpiry      time.Duration
}

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := updateOptions{autoAccept: NewAutoAcceptOption()}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the Swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.autoAccept, flagAutoAccept, "Auto acceptance policy (worker, manager or none)")
	flags.StringVar(&opts.secret, flagSecret, "", "Set secret value needed to accept nodes into cluster")
	flags.Int64Var(&opts.taskHistoryLimit, flagTaskHistoryLimit, 10, "Task history retention limit")
	flags.DurationVar(&opts.dispatcherHeartbeat, flagDispatcherHeartbeat, time.Duration(5*time.Second), "Dispatcher heartbeat period")
	flags.DurationVar(&opts.nodeCertExpiry, flagCertExpiry, time.Duration(90*24*time.Hour), "Validity period for node certificates")
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, flags *pflag.FlagSet, opts updateOptions) error {
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
