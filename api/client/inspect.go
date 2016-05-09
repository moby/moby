package client

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client/inspect"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils/templates"
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

	var elementSearcher inspectSearcher
	switch *inspectType {
	case "container":
		elementSearcher = cli.inspectContainers(*size)
	case "image":
		elementSearcher = cli.inspectImages(*size)
	default:
		elementSearcher = cli.inspectAll(*size)
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), elementSearcher)
}

func (cli *DockerCli) inspectContainers(getSize bool) inspectSearcher {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ContainerInspectWithRaw(context.Background(), ref, getSize)
	}
}

func (cli *DockerCli) inspectImages(getSize bool) inspectSearcher {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ImageInspectWithRaw(context.Background(), ref, getSize)
	}
}

func (cli *DockerCli) inspectAll(getSize bool) inspectSearcher {
	return func(ref string) (interface{}, []byte, error) {
		c, rawContainer, err := cli.client.ContainerInspectWithRaw(context.Background(), ref, getSize)
		if err != nil {
			// Search for image with that id if a container doesn't exist.
			if client.IsErrContainerNotFound(err) {
				i, rawImage, err := cli.client.ImageInspectWithRaw(context.Background(), ref, getSize)
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

type inspectSearcher func(ref string) (interface{}, []byte, error)

func (cli *DockerCli) inspectElements(tmplStr string, references []string, searchByReference inspectSearcher) error {
	elementInspector, err := cli.newInspectorWithTemplate(tmplStr)
	if err != nil {
		return Cli.StatusError{StatusCode: 64, Status: err.Error()}
	}

	var inspectErr error
	for _, ref := range references {
		element, raw, err := searchByReference(ref)
		if err != nil {
			inspectErr = err
			break
		}

		if err := elementInspector.Inspect(element, raw); err != nil {
			inspectErr = err
			break
		}
	}

	if err := elementInspector.Flush(); err != nil {
		cli.inspectErrorStatus(err)
	}

	if status := cli.inspectErrorStatus(inspectErr); status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) inspectErrorStatus(err error) (status int) {
	if err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		status = 1
	}
	return
}

func (cli *DockerCli) newInspectorWithTemplate(tmplStr string) (inspect.Inspector, error) {
	elementInspector := inspect.NewIndentedInspector(cli.out)
	if tmplStr != "" {
		tmpl, err := templates.Parse(tmplStr)
		if err != nil {
			return nil, fmt.Errorf("Template parsing error: %s", err)
		}
		elementInspector = inspect.NewTemplateInspector(cli.out, tmpl)
	}
	return elementInspector, nil
}
