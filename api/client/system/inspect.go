package system

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	apiclient "github.com/docker/engine-api/client"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format      string
	inspectType string
	size        bool
	ids         []string
}

// NewInspectCommand creates a new cobra.Command for `docker inspect`
func NewInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
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
	flags.StringVar(&opts.inspectType, "type", "", "Return JSON for specified type, (e.g image, container or task)")
	flags.BoolVarP(&opts.size, "size", "s", false, "Display total file sizes if the type is container")

	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	ctx := context.Background()
	client := dockerCli.Client()

	var getRefFunc inspect.GetRefFunc
	switch opts.inspectType {
	case "container":
		getRefFunc = func(ref string) (interface{}, []byte, error) {
			return client.ContainerInspectWithRaw(ctx, ref, opts.size)
		}
	case "image":
		getRefFunc = func(ref string) (interface{}, []byte, error) {
			return client.ImageInspectWithRaw(ctx, ref)
		}
	case "task":
		if opts.size {
			fmt.Fprintln(dockerCli.Err(), "WARNING: --size ignored for tasks")
		}
		getRefFunc = func(ref string) (interface{}, []byte, error) {
			return client.TaskInspectWithRaw(ctx, ref)
		}
	case "":
		getRefFunc = inspectAll(ctx, dockerCli, opts.size)
	default:
		return fmt.Errorf("%q is not a valid value for --type", opts.inspectType)
	}

	return inspect.Inspect(dockerCli.Out(), opts.ids, opts.format, getRefFunc)
}

func inspectAll(ctx context.Context, dockerCli *client.DockerCli, getSize bool) inspect.GetRefFunc {
	client := dockerCli.Client()

	return func(ref string) (interface{}, []byte, error) {
		c, rawContainer, err := client.ContainerInspectWithRaw(ctx, ref, getSize)
		if err == nil || !apiclient.IsErrNotFound(err) {
			return c, rawContainer, err
		}
		// Search for image with that id if a container doesn't exist.
		i, rawImage, err := client.ImageInspectWithRaw(ctx, ref)
		if err == nil || !apiclient.IsErrNotFound(err) {
			return i, rawImage, err
		}

		// Search for task with that id if an image doesn't exist.
		t, rawTask, err := client.TaskInspectWithRaw(ctx, ref)
		if err == nil || !(apiclient.IsErrNotFound(err) || isErrorNoSwarmMode(err)) {
			if getSize {
				fmt.Fprintln(dockerCli.Err(), "WARNING: --size ignored for tasks")
			}
			return t, rawTask, err
		}
		return nil, nil, fmt.Errorf("Error: No such container, image or task: %s", ref)
	}
}

func isErrorNoSwarmMode(err error) bool {
	return strings.Contains(err.Error(), "This node is not a swarm manager")
}
