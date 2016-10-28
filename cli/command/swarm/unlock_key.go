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

func newUnlockKeyCommand(dockerCli *command.DockerCli) *cobra.Command {
	var rotate, quiet bool

	cmd := &cobra.Command{
		Use:   "unlock-key [OPTIONS]",
		Short: "Manage the unlock key",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := dockerCli.Client()
			ctx := context.Background()

			if rotate {
				flags := swarm.UpdateFlags{RotateManagerUnlockKey: true}

				swarm, err := client.SwarmInspect(ctx)
				if err != nil {
					return err
				}

				if !swarm.Spec.EncryptionConfig.AutoLockManagers {
					return errors.New("cannot rotate because autolock is not turned on")
				}

				err = client.SwarmUpdate(ctx, swarm.Version, swarm.Spec, flags)
				if err != nil {
					return err
				}
				if !quiet {
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

			if quiet {
				fmt.Fprintln(dockerCli.Out(), unlockKeyResp.UnlockKey)
			} else {
				printUnlockCommand(ctx, dockerCli, unlockKeyResp.UnlockKey)
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&rotate, flagRotate, false, "Rotate unlock key")
	flags.BoolVarP(&quiet, flagQuiet, "q", false, "Only display token")

	return cmd
}

func printUnlockCommand(ctx context.Context, dockerCli *command.DockerCli, unlockKey string) {
	if len(unlockKey) == 0 {
		return
	}

	fmt.Fprintf(dockerCli.Out(), "To unlock a swarm manager after it restarts, run the `docker swarm unlock`\ncommand and provide the following key:\n\n    %s\n\nPlease remember to store this key in a password manager, since without it you\nwill not be able to restart the manager.\n", unlockKey)
	return
}
