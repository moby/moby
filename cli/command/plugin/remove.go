// +build experimental

package plugin

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/reference"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type rmOptions struct {
	force bool

	plugins []string
}

func newRemoveCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts rmOptions

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] PLUGIN [PLUGIN...]",
		Short:   "Remove one or more plugins",
		Aliases: []string{"remove"},
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.plugins = args
			return runRemove(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Force the removal of an active plugin")
	return cmd
}

func runRemove(dockerCli *command.DockerCli, opts *rmOptions) error {
	ctx := context.Background()

	var errs cli.Errors
	for _, name := range opts.plugins {
		named, err := reference.ParseNamed(name) // FIXME: validate
		if err != nil {
			return err
		}
		if reference.IsNameOnly(named) {
			named = reference.WithDefaultTag(named)
		}
		ref, ok := named.(reference.NamedTagged)
		if !ok {
			return fmt.Errorf("invalid name: %s", named.String())
		}
		// TODO: pass names to api instead of making multiple api calls
		if err := dockerCli.Client().PluginRemove(ctx, ref.String(), types.PluginRemoveOptions{Force: opts.force}); err != nil {
			errs = append(errs, err)
			continue
		}
		fmt.Fprintln(dockerCli.Out(), name)
	}
	// Do not simplify to `return errs` because even if errs == nil, it is not a nil-error interface value.
	if errs != nil {
		return errs
	}
	return nil
}
