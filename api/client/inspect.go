package client

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client/inspect"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/client"
)

// CmdInspect displays low-level information on one or more containers, images or tasks.
//
// Usage: docker inspect [OPTIONS] CONTAINER|IMAGE|TASK [CONTAINER|IMAGE|TASK...]
func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := Cli.Subcmd("inspect", []string{"[OPTIONS] CONTAINER|IMAGE|TASK [CONTAINER|IMAGE|TASK...]"}, Cli.DockerCommands["inspect"].Description, true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "Format the output using the given go template")
	inspectType := cmd.String([]string{"-type"}, "", "Return JSON for specified type, (e.g image, container or task)")
	size := cmd.Bool([]string{"s", "-size"}, false, "Display total file sizes if the type is container")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *inspectType != "" && *inspectType != "container" && *inspectType != "image" && *inspectType != "task" {
		return fmt.Errorf("%q is not a valid value for --type", *inspectType)
	}

	ctx := context.Background()

	var elementSearcher inspect.GetRefFunc
	switch *inspectType {
	case "container":
		elementSearcher = cli.inspectContainers(ctx, *size)
	case "image":
		elementSearcher = cli.inspectImages(ctx, *size)
	case "task":
		if *size {
			fmt.Fprintln(cli.err, "WARNING: --size ignored for tasks")
		}
		elementSearcher = cli.inspectTasks(ctx)
	default:
		elementSearcher = cli.inspectAll(ctx, *size)
	}

	return inspect.Inspect(cli.out, cmd.Args(), *tmplStr, elementSearcher)
}

func (cli *DockerCli) inspectContainers(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ContainerInspectWithRaw(ctx, ref, getSize)
	}
}

func (cli *DockerCli) inspectImages(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ImageInspectWithRaw(ctx, ref, getSize)
	}
}

func (cli *DockerCli) inspectTasks(ctx context.Context) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.TaskInspectWithRaw(ctx, ref)
	}
}

func (cli *DockerCli) inspectAll(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		c, rawContainer, err := cli.client.ContainerInspectWithRaw(ctx, ref, getSize)
		if err != nil {
			// Search for image with that id if a container doesn't exist.
			if client.IsErrContainerNotFound(err) {
				i, rawImage, err := cli.client.ImageInspectWithRaw(ctx, ref, getSize)
				if err != nil {
					if client.IsErrImageNotFound(err) {
						// Search for task with that id if an image doesn't exists.
						t, rawTask, err := cli.client.TaskInspectWithRaw(ctx, ref)
						if err != nil {
							return nil, nil, fmt.Errorf("Error: No such image, container or task: %s", ref)
						}
						if getSize {
							fmt.Fprintln(cli.err, "WARNING: --size ignored for tasks")
						}
						return t, rawTask, nil
					}
					return nil, nil, err
				}
				return i, rawImage, nil
			}
			return nil, nil, err
		}
		return c, rawContainer, nil
	}
}
