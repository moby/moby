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
		Use:   "inspect [OPTIONS] CONTAINER|IMAGE|TASK [CONTAINER|IMAGE|TASK...]",
		Short: "Return low-level information on a container, image or task",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ids = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	flags.StringVar(&opts.inspectType, "type", "", "Return JSON for specified type")
	flags.BoolVarP(&opts.size, "size", "s", false, "Display total file sizes if the type is container")

	return cmd
}

func runInspect(dockerCli *command.DockerCli, opts inspectOptions) error {
	var elementSearcher inspect.GetRefFunc
	switch opts.inspectType {
	case "", "container", "image", "node", "network", "service", "volume", "task":
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

func inspectAll(ctx context.Context, dockerCli *command.DockerCli, getSize bool, typeConstraint string) inspect.GetRefFunc {
	var inspectAutodetect = []struct {
		ObjectType      string
		IsSizeSupported bool
		ObjectInspector func(string) (interface{}, []byte, error)
	}{
		{"container", true, inspectContainers(ctx, dockerCli, getSize)},
		{"image", false, inspectImages(ctx, dockerCli)},
		{"network", false, inspectNetwork(ctx, dockerCli)},
		{"volume", false, inspectVolume(ctx, dockerCli)},
		{"service", false, inspectService(ctx, dockerCli)},
		{"task", false, inspectTasks(ctx, dockerCli)},
		{"node", false, inspectNode(ctx, dockerCli)},
	}

	isErrNotSwarmManager := func(err error) bool {
		return strings.Contains(err.Error(), "This node is not a swarm manager")
	}

	return func(ref string) (interface{}, []byte, error) {
		for _, inspectData := range inspectAutodetect {
			if typeConstraint != "" && inspectData.ObjectType != typeConstraint {
				continue
			}
			v, raw, err := inspectData.ObjectInspector(ref)
			if err != nil {
				if typeConstraint == "" && (apiclient.IsErrNotFound(err) || isErrNotSwarmManager(err)) {
					continue
				}
				return v, raw, err
			}
			if getSize && !inspectData.IsSizeSupported {
				fmt.Fprintf(dockerCli.Err(), "WARNING: --size ignored for %s\n", inspectData.ObjectType)
			}
			return v, raw, err
		}
		return nil, nil, fmt.Errorf("Error: No such object: %s", ref)
	}
}
