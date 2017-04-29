package volume

import (
	"fmt"
	"strings"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type removeOptions struct {
	force bool

	volumes []string
}

func newRemoveCommand(dockerCli command.Cli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] VOLUME [VOLUME...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more volumes",
		Long:    removeDescription,
		Example: removeExample,
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.volumes = args
			return runRemove(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.force, "force", "f", false, "Force the removal of one or more volumes")
	flags.SetAnnotation("force", "version", []string{"1.25"})
	return cmd
}

func runRemove(dockerCli command.Cli, opts *removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	var errs []string

	for _, name := range opts.volumes {
		if err := client.VolumeRemove(ctx, name, opts.force); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", name)
	}

	if len(errs) > 0 {
		return errors.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

var removeDescription = `
Remove one or more volumes. You cannot remove a volume that is in use by a container.
`

var removeExample = `
$ docker volume rm hello
hello
`
