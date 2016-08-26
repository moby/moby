package swarm

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newUpdateCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := swarmOptions{}

	cmd := &cobra.Command{
		Use:   "update [OPTIONS]",
		Short: "Update the swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), opts)
		},
	}

	addSwarmFlags(cmd.Flags(), &opts)
	return cmd
}

func runUpdate(dockerCli *command.DockerCli, flags *pflag.FlagSet, opts swarmOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	var updateFlags swarm.UpdateFlags

	swarm, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	err = mergeSwarm(&swarm, flags)
	if err != nil {
		return err
	}

	err = client.SwarmUpdate(ctx, swarm.Version, swarm.Spec, updateFlags)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), "Swarm updated.")

	return nil
}

func mergeSwarm(swarm *swarm.Swarm, flags *pflag.FlagSet) error {
	spec := &swarm.Spec

	if flags.Changed(flagTaskHistoryLimit) {
		taskHistoryRetentionLimit, _ := flags.GetInt64(flagTaskHistoryLimit)
		spec.Orchestration.TaskHistoryRetentionLimit = &taskHistoryRetentionLimit
	}

	if flags.Changed(flagDispatcherHeartbeat) {
		if v, err := flags.GetDuration(flagDispatcherHeartbeat); err == nil {
			spec.Dispatcher.HeartbeatPeriod = v
		}
	}

	if flags.Changed(flagCertExpiry) {
		if v, err := flags.GetDuration(flagCertExpiry); err == nil {
			spec.CAConfig.NodeCertExpiry = v
		}
	}

	if flags.Changed(flagExternalCA) {
		value := flags.Lookup(flagExternalCA).Value.(*ExternalCAOption)
		spec.CAConfig.ExternalCAs = value.Value()
	}

	return nil
}
