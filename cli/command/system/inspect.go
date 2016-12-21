package system

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	apiclient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format      string
	inspectType string
	size        bool
	ids         []string
}

// NewInspectCommand creates a new cobra.Command for `docker inspect`
func NewInspectCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] NAME|ID [NAME|ID...]",
		Short: "Return low-level information on Docker objects",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ids = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given Go template")
	flags.StringVar(&opts.inspectType, "type", "", "Return JSON for specified type")
	flags.BoolVarP(&opts.size, "size", "s", false, "Display total file sizes if the type is container")

	return cmd
}

func runInspect(dockerCli *command.DockerCli, opts inspectOptions) error {
	var elementSearcher inspect.GetRefFunc
	switch opts.inspectType {
	case "", "container", "image", "node", "network", "service", "volume", "task", "plugin":
		elementSearcher = inspectAll(context.Background(), dockerCli, opts.size, opts.inspectType)
	default:
		return fmt.Errorf("%q is not a valid value for --type", opts.inspectType)
	}
	return inspect.Inspect(dockerCli.Out(), opts.ids, opts.format, elementSearcher)
}

func inspectContainers(ctx context.Context, dockerCli *command.DockerCli, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ContainerInspectWithRaw(ctx, ref, getSize)
	}
}

func inspectImages(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ImageInspectWithRaw(ctx, ref)
	}
}

func inspectNetwork(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().NetworkInspectWithRaw(ctx, ref)
	}
}

func inspectNode(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().NodeInspectWithRaw(ctx, ref)
	}
}

func inspectService(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ServiceInspectWithRaw(ctx, ref)
	}
}

func inspectTasks(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().TaskInspectWithRaw(ctx, ref)
	}
}

func inspectVolume(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().VolumeInspectWithRaw(ctx, ref)
	}
}

func inspectPlugin(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().PluginInspectWithRaw(ctx, ref)
	}
}

func inspectAll(ctx context.Context, dockerCli *command.DockerCli, getSize bool, typeConstraint string) inspect.GetRefFunc {
	var inspectAutodetect = []struct {
		objectType      string
		isSizeSupported bool
		isSwarmObject   bool
		objectInspector func(string) (interface{}, []byte, error)
	}{
		{
			objectType:      "container",
			isSizeSupported: true,
			objectInspector: inspectContainers(ctx, dockerCli, getSize),
		},
		{
			objectType:      "image",
			objectInspector: inspectImages(ctx, dockerCli),
		},
		{
			objectType:      "network",
			objectInspector: inspectNetwork(ctx, dockerCli),
		},
		{
			objectType:      "volume",
			objectInspector: inspectVolume(ctx, dockerCli),
		},
		{
			objectType:      "service",
			isSwarmObject:   true,
			objectInspector: inspectService(ctx, dockerCli),
		},
		{
			objectType:      "task",
			isSwarmObject:   true,
			objectInspector: inspectTasks(ctx, dockerCli),
		},
		{
			objectType:      "node",
			isSwarmObject:   true,
			objectInspector: inspectNode(ctx, dockerCli),
		},
		{
			objectType:      "plugin",
			objectInspector: inspectPlugin(ctx, dockerCli),
		},
	}

	// isSwarmManager does an Info API call to verify that the daemon is
	// a swarm manager.
	isSwarmManager := func() bool {
		info, err := dockerCli.Client().Info(ctx)
		if err != nil {
			fmt.Fprintln(dockerCli.Err(), err)
			return false
		}
		return info.Swarm.ControlAvailable
	}

	isErrNotSupported := func(err error) bool {
		return strings.Contains(err.Error(), "not supported")
	}

	return func(ref string) (interface{}, []byte, error) {
		const (
			swarmSupportUnknown = iota
			swarmSupported
			swarmUnsupported
		)

		isSwarmSupported := swarmSupportUnknown

		for _, inspectData := range inspectAutodetect {
			if typeConstraint != "" && inspectData.objectType != typeConstraint {
				continue
			}
			if typeConstraint == "" && inspectData.isSwarmObject {
				if isSwarmSupported == swarmSupportUnknown {
					if isSwarmManager() {
						isSwarmSupported = swarmSupported
					} else {
						isSwarmSupported = swarmUnsupported
					}
				}
				if isSwarmSupported == swarmUnsupported {
					continue
				}
			}
			v, raw, err := inspectData.objectInspector(ref)
			if err != nil {
				if typeConstraint == "" && (apiclient.IsErrNotFound(err) || isErrNotSupported(err)) {
					continue
				}
				return v, raw, err
			}
			if getSize && !inspectData.isSizeSupported {
				fmt.Fprintf(dockerCli.Err(), "WARNING: --size ignored for %s\n", inspectData.objectType)
			}
			return v, raw, err
		}
		return nil, nil, fmt.Errorf("Error: No such object: %s", ref)
	}
}
