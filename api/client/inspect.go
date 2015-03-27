package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/docker/docker/i18n"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// CmdInspect displays low-level information on one or more containers or images.
//
// Usage: docker inspect [OPTIONS] CONTAINER|IMAGE [CONTAINER|IMAGE...]
func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := cli.Subcmd("inspect", "CONTAINER|IMAGE [CONTAINER|IMAGE...]", "Return low-level information on a container or image", true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var tmpl *template.Template
	if *tmplStr != "" {
		var err error
		if tmpl, err = template.New("").Funcs(funcMap).Parse(*tmplStr); err != nil {
			fmt.Fprintf(cli.err, "Template parsing error: %v\n", err)
			return &utils.StatusError{StatusCode: 64,
				Status: "Template parsing error: " + err.Error()}
		}
	}

	indented := new(bytes.Buffer)
	indented.WriteByte('[')
	status := 0

	for _, name := range cmd.Args() {
		obj, _, err := readBody(cli.call("GET", "/containers/"+name+"/json", nil, nil))
		if err != nil {
			if i18n.Is(err, "NoContainerID") {
				obj, _, err = readBody(cli.call("GET", "/images/"+name+"/json", nil, nil))
				if i18n.Is(err, i18n.NoImageID) {
					err = i18n.NewError(i18n.NoContainerImageID, name)
				}
			}
		}
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}

		if tmpl == nil {
			if err = json.Indent(indented, obj, "", "    "); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
		} else {
			// Has template, will render
			var value interface{}
			if err := json.Unmarshal(obj, &value); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
			if err := tmpl.Execute(cli.out, value); err != nil {
				return err
			}
			cli.out.Write([]byte{'\n'})
		}
		indented.WriteString(",")
	}

	if indented.Len() > 1 {
		// Remove trailing ','
		indented.Truncate(indented.Len() - 1)
	}
	indented.WriteString("]\n")

	if tmpl == nil {
		if _, err := io.Copy(cli.out, indented); err != nil {
			return err
		}
	}

	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}
	return nil
}
