package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"

	"github.com/docker/docker/api/types"
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
	cmd := Cli.Subcmd("inspect", []string{"CONTAINER|IMAGE [CONTAINER|IMAGE...]"}, "Return low-level information on a container or image", true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template")
	inspectType := cmd.String([]string{"-type"}, "", "Return JSON for specified type, (e.g image or container)")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *inspectType != "" && *inspectType != "container" && *inspectType != "image" {
		return fmt.Errorf("%q is not a valid value for --type", *inspectType)
	}

	if *tmplStr != "" {
		tmpl, err := template.New("").Funcs(funcMap).Parse(*tmplStr)
		if err != nil {
			return Cli.StatusError{StatusCode: 64,
				Status: "Template parsing error: " + err.Error()}
		}
		return cli.inspectWithTemplate(tmpl, *inspectType, cmd.Args()...)
	}

	var status int
	buf := bytes.NewBuffer(nil)
	io.WriteString(buf, "[")
	for i, name := range cmd.Args() {
		obj, err := cli.inspect(*inspectType, name)
		if err != nil {
			status = 1
			fmt.Fprintf(cli.err, "%v\n", err)
			continue
		}
		b, err := json.MarshalIndent(obj, "", "    ")
		if err != nil {
			status = 1
			fmt.Fprintf(cli.err, "%v\n", err)
			continue
		}
		io.WriteString(buf, "\n")
		buf.Write(b)
		if i < len(cmd.Args())-1 {
			io.WriteString(buf, ",")
		} else {
			io.WriteString(buf, "\n")
		}
	}
	io.WriteString(buf, "]")

	_, err := io.Copy(cli.out, buf)
	if err != nil {
		return err
	}
	io.WriteString(cli.out, "\n")

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) inspectWithTemplate(tmpl *template.Template, inspectType string, names ...string) error {
	var status int
	for _, name := range names {
		obj, err := cli.inspect(inspectType, name)
		if err != nil {
			status = 1
			fmt.Fprintf(cli.err, "%v\n", err)
			continue
		}
		if err := tmpl.Execute(cli.out, &obj); err != nil {
			status = 1
			fmt.Fprintln(cli.err, err)
			continue
		}
		io.WriteString(cli.out, "\n")
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) inspect(inspectType, name string) (interface{}, error) {
	var out interface{}
	var statusCode int
	var err error

	switch inspectType {
	case "container":
		out, statusCode, err = cli.inspectContainer(name)
		if err != nil && statusCode == http.StatusNotFound {
			return nil, fmt.Errorf("No such container: %s", name)
		}
	case "image":
		out, statusCode, err = cli.inspectImage(name)
		if err != nil && statusCode == http.StatusNotFound {
			return nil, fmt.Errorf("No such image: %s", name)
		}
	case "":
		out, statusCode, err = cli.inspectContainer(name)
		if err != nil && statusCode == http.StatusNotFound {
			out, statusCode, err = cli.inspectImage(name)
			if err != nil && statusCode == http.StatusNotFound {
				return nil, fmt.Errorf("No such image or container: %s", name)
			}
		}
	}
	return out, err
}

func (cli *DockerCli) inspectContainer(name string) (*types.ContainerJSON, int, error) {
	resp, err := cli.call("GET", "/containers/"+name+"/json", nil, nil)
	if err != nil {
		return nil, resp.statusCode, err
	}
	defer resp.body.Close()
	container := &types.ContainerJSON{}

	dec := json.NewDecoder(resp.body)
	if err := dec.Decode(container); err != nil {
		return nil, -1, err
	}
	return container, resp.statusCode, nil
}

func (cli *DockerCli) inspectImage(name string) (*types.ImageInspect, int, error) {
	resp, err := cli.call("GET", "/images/"+name+"/json", nil, nil)
	if err != nil {
		return nil, resp.statusCode, err
	}
	defer resp.body.Close()
	image := &types.ImageInspect{}

	dec := json.NewDecoder(resp.body)
	if err := dec.Decode(image); err != nil {
		return nil, -1, err
	}
	return image, resp.statusCode, nil
}
