package swarm

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newUpdateCommand(dockerCli command.Cli) *cobra.Command {
	opts := swarmOptions{}

	cmd := &cobra.Command{
		Use:   "update [OPTIONS]",
		Short: "Update the swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), opts)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return pflag.ErrHelp
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.autolock, flagAutolock, false, "Change manager autolocking setting (true|false)")
	addSwarmFlags(cmd.Flags(), &opts)
	return cmd
}

func runUpdate(dockerCli command.Cli, flags *pflag.FlagSet, opts swarmOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	var updateFlags swarm.UpdateFlags

	swarmInspect, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	prevAutoLock := swarmInspect.Spec.EncryptionConfig.AutoLockManagers

	opts.mergeSwarmSpec(&swarmInspect.Spec, flags)

	curAutoLock := swarmInspect.Spec.EncryptionConfig.AutoLockManagers

	err = client.SwarmUpdate(ctx, swarmInspect.Version, swarmInspect.Spec, updateFlags)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), "Swarm updated.")

	if curAutoLock && !prevAutoLock {
		unlockKeyResp, err := client.SwarmGetUnlockKey(ctx)
		if err != nil {
			return errors.Wrap(err, "could not fetch unlock key")
		}
		printUnlockCommand(ctx, dockerCli, unlockKeyResp.UnlockKey)
	}

	return nil
}
