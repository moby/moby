package volume

import (
	"context"
	"fmt"

	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	name       string
	driverOpts opts.MapOpts
	labels     opts.ListOpts
}

func newUpdateCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := updateOptions{
		driverOpts: *opts.NewMapOpts(nil, nil),
		labels:     opts.NewListOpts(opts.ValidateEnv),
	}

	cmd := &cobra.Command{
		Use:   "update OPTIONS VOLUME",
		Short: "Update a volume",
		Long:  updateDescription,
		Args:  cli.RequiresMaxArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				if opts.name != "" {
					fmt.Fprint(dockerCli.Err(), "Conflicting options: either specify --name or provide positional arg, not both\n")
					return cli.StatusError{StatusCode: 1}
				}
				opts.name = args[1]
			} else {
				if opts.name == "" {
					fmt.Fprint(dockerCli.Err(), "Insufficient options: either specify --name or provide positional arg\n")
					return cli.StatusError{StatusCode: 1}
				}
			}
			return runUpdate(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.name, "name", "", "Specify volume name")
	flags.VarP(&opts.driverOpts, "opt", "o", "Set driver specific options to update")
	flags.Var(&opts.labels, "label", "Set label metadata to update")

	return cmd
}

func runUpdate(dockerCli *command.DockerCli, opts updateOptions) error {
	client := dockerCli.Client()

	volReq := volumetypes.VolumesUpdateBody{
		DriverOpts: opts.driverOpts.GetAll(),
		Labels:     runconfigopts.ConvertKVStringsToMap(opts.labels.GetAll()),
	}

	vol, err := client.VolumeUpdate(context.Background(), opts.name, volReq)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", vol.Name)
	return nil
}

var updateDescription = `
Update the driver specific options on a volume.
This allows modification of volume options after they are created.
For example;

    $ docker volume create --name hello -o device=tmpfs -o type=tmpfs -o o="size=10m"
    hello
    $ docker volume update --name hello -o o="size=100m"
	hello

## Driver specific options

Some volume drivers may take options to customize the volume. Use the
**-o** or **--opt** flags to pass driver options:

    $ docker volume update --driver fake --opt tardis=blue --opt timey=wimey

These options are passed directly to the volume driver. Options for different
volume drivers may do different things (or nothing at all).

The built-in **local** driver on Windows does not support any options.

The built-in **local** driver on Linux accepts options similar to the linux
**mount** command.  This driver attempts to mount the volume before committing the
requested changes.  For example:

    $ docker volume update --driver local --opt o=size=100m,uid=1000

Another example:

    $ docker volume update --driver local --opt device=/dev/sda2

`
