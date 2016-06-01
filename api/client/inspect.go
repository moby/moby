package client

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client/inspect"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/client"
)

// CmdInspect displays low-level information on one or more containers or images.
//
// Usage: docker inspect [OPTIONS] CONTAINER|IMAGE [CONTAINER|IMAGE...]
func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := Cli.Subcmd("inspect", []string{"CONTAINER|IMAGE [CONTAINER|IMAGE...]"}, Cli.DockerCommands["inspect"].Description, true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "Format the output using the given go template")
	inspectType := cmd.String([]string{"-type"}, "", "Return JSON for specified type, (e.g image or container)")
	size := cmd.Bool([]string{"s", "-size"}, false, "Display total file sizes if the type is container")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *inspectType != "" && *inspectType != "container" && *inspectType != "image" {
		return fmt.Errorf("%q is not a valid value for --type", *inspectType)
	}

	ctx := context.Background()

	var elementSearcher inspect.GetRefFunc
	switch *inspectType {
	case "container":
		elementSearcher = cli.inspectContainers(ctx, *size)
	case "image":
		elementSearcher = cli.inspectImages(ctx, *size)
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

func (cli *DockerCli) inspectAll(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		c, rawContainer, err := cli.client.ContainerInspectWithRaw(ctx, ref, getSize)
		if err != nil {
			// Search for image with that id if a container doesn't exist.
			if client.IsErrContainerNotFound(err) {
				i, rawImage, err := cli.client.ImageInspectWithRaw(ctx, ref, getSize)
				if err != nil {
					if client.IsErrImageNotFound(err) {
						return nil, nil, fmt.Errorf("Error: No such image or container: %s", ref)
					}
					return nil, nil, err
				}
				return i, rawImage, err
			}
			return nil, nil, err
		}
		return c, rawContainer, err
	}
}
