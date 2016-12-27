package swarm

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type unlockKeyOptions struct {
	rotate bool
	quiet  bool
}

func newUnlockKeyCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := unlockKeyOptions{}

	cmd := &cobra.Command{
		Use:   "unlock-key [OPTIONS]",
		Short: "Manage the unlock key",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnlockKey(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.rotate, flagRotate, false, "Rotate unlock key")
	flags.BoolVarP(&opts.quiet, flagQuiet, "q", false, "Only display token")

	return cmd
}

func runUnlockKey(dockerCli *command.DockerCli, opts unlockKeyOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	if opts.rotate {
		flags := swarm.UpdateFlags{RotateManagerUnlockKey: true}

		sw, err := client.SwarmInspect(ctx)
		if err != nil {
			return err
		}

		if !sw.Spec.EncryptionConfig.AutoLockManagers {
			return errors.New("cannot rotate because autolock is not turned on")
		}

		if err := client.SwarmUpdate(ctx, sw.Version, sw.Spec, flags); err != nil {
			return err
		}

		if !opts.quiet {
			fmt.Fprintf(dockerCli.Out(), "Successfully rotated manager unlock key.\n\n")
		}
	}

	unlockKeyResp, err := client.SwarmGetUnlockKey(ctx)
	if err != nil {
		return errors.Wrap(err, "could not fetch unlock key")
	}

	if unlockKeyResp.UnlockKey == "" {
		return errors.New("no unlock key is set")
	}

	if opts.quiet {
		fmt.Fprintln(dockerCli.Out(), unlockKeyResp.UnlockKey)
		return nil
	}

	printUnlockCommand(ctx, dockerCli, unlockKeyResp.UnlockKey)
	return nil
}

func printUnlockCommand(ctx context.Context, dockerCli *command.DockerCli, unlockKey string) {
	if len(unlockKey) > 0 {
		fmt.Fprintf(dockerCli.Out(), "To unlock a swarm manager after it restarts, run the `docker swarm unlock`\ncommand and provide the following key:\n\n    %s\n\nPlease remember to store this key in a password manager, since without it you\nwill not be able to restart the manager.\n", unlockKey)
	}
	return
}
