package client

import (
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/api/client/lib"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
}

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

	var (
		err              error
		tmpl             *template.Template
		elementInspector inspect.Inspector
	)

	if *tmplStr != "" {
		if tmpl, err = template.New("").Funcs(funcMap).Parse(*tmplStr); err != nil {
			return Cli.StatusError{StatusCode: 64,
				Status: "Template parsing error: " + err.Error()}
		}
	}

	if tmpl != nil {
		elementInspector = inspect.NewTemplateInspector(cli.out, tmpl)
	} else {
		elementInspector = inspect.NewIndentedInspector(cli.out)
	}

	switch *inspectType {
	case "container":
		err = cli.inspectContainers(cmd.Args(), *size, elementInspector)
	case "images":
		err = cli.inspectImages(cmd.Args(), *size, elementInspector)
	default:
		err = cli.inspectAll(cmd.Args(), *size, elementInspector)
	}

	if err := elementInspector.Flush(); err != nil {
		return err
	}
	return err
}

func (cli *DockerCli) inspectContainers(containerIDs []string, getSize bool, elementInspector inspect.Inspector) error {
	for _, containerID := range containerIDs {
		if err := cli.inspectContainer(containerID, getSize, elementInspector); err != nil {
			if lib.IsErrContainerNotFound(err) {
				return fmt.Errorf("Error: No such container: %s\n", containerID)
			}
			return err
		}
	}
	return nil
}

func (cli *DockerCli) inspectImages(imageIDs []string, getSize bool, elementInspector inspect.Inspector) error {
	for _, imageID := range imageIDs {
		if err := cli.inspectImage(imageID, getSize, elementInspector); err != nil {
			if lib.IsErrImageNotFound(err) {
				return fmt.Errorf("Error: No such image: %s\n", imageID)
			}
			return err
		}
	}
	return nil
}

func (cli *DockerCli) inspectAll(ids []string, getSize bool, elementInspector inspect.Inspector) error {
	for _, id := range ids {
		if err := cli.inspectContainer(id, getSize, elementInspector); err != nil {
			// Search for image with that id if a container doesn't exist.
			if lib.IsErrContainerNotFound(err) {
				if err := cli.inspectImage(id, getSize, elementInspector); err != nil {
					if lib.IsErrImageNotFound(err) {
						return fmt.Errorf("Error: No such image or container: %s", id)
					}
					return err
				}
				continue
			}
			return err
		}
	}
	return nil
}

func (cli *DockerCli) inspectContainer(containerID string, getSize bool, elementInspector inspect.Inspector) error {
	c, raw, err := cli.client.ContainerInspectWithRaw(containerID, getSize)
	if err != nil {
		return err
	}

	return elementInspector.Inspect(c, raw)
}

func (cli *DockerCli) inspectImage(imageID string, getSize bool, elementInspector inspect.Inspector) error {
	i, raw, err := cli.client.ImageInspectWithRaw(imageID, getSize)
	if err != nil {
		return err
	}

	return elementInspector.Inspect(i, raw)
}
