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

type updateOptions struct {
	autoAccept       AutoAcceptOption
	secret           string
	taskHistoryLimit int64
	heartbeatPeriod  uint64
}

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := updateOptions{autoAccept: NewAutoAcceptOption()}
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "update",
		Short: "update the Swarm.",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, flags, opts)
		},
	}

	flags = cmd.Flags()
	flags.Var(&opts.autoAccept, "auto-accept", "Acceptance policy")
	flags.StringVar(&opts.secret, "secret", "", "Set secret value needed to accept nodes into cluster")
	flags.Int64Var(&opts.taskHistoryLimit, "task-history-limit", 10, "Task history retention limit")
	flags.Uint64Var(&opts.heartbeatPeriod, "dispatcher-heartbeat-period", 5000000000, "Dispatcher heartbeat period")
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
	err = client.SwarmUpdate(ctx, swarm)
	if err != nil {
		return err
	}

	fmt.Println("Swarm updated.")
	return nil
}

func mergeSwarm(swarm *swarm.Swarm, flags *pflag.FlagSet) error {
	spec := &swarm.Spec

	if flags.Changed("auto-accept") {
		value := flags.Lookup("auto-accept").Value.(*AutoAcceptOption)
		if len(spec.AcceptancePolicy.Policies) > 0 {
			spec.AcceptancePolicy.Policies = value.Policies(spec.AcceptancePolicy.Policies[0].Secret)
		} else {
			spec.AcceptancePolicy.Policies = value.Policies("")
		}
	}

	if flags.Changed("secret") {
		secret, _ := flags.GetString("secret")
		for _, policy := range spec.AcceptancePolicy.Policies {
			policy.Secret = secret
		}
	}

	if flags.Changed("task-history-limit") {
		spec.Orchestration.TaskHistoryRetentionLimit, _ = flags.GetInt64("task-history-limit")
	}

	if flags.Changed("dispatcher-heartbeat-period") {
		spec.Dispatcher.HeartbeatPeriod, _ = flags.GetUint64("dispatcher-heartbeat-period")
	}

	return nil
}
