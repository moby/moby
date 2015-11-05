package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
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
	cmd := Cli.Subcmd("inspect", []string{"CONTAINER|IMAGE [CONTAINER|IMAGE...]"}, Cli.DockerCommands["inspect"].Description, true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template")
	inspectType := cmd.String([]string{"-type"}, "", "Return JSON for specified type, (e.g image or container)")
	size := cmd.Bool([]string{"s", "-size"}, false, "Display total file sizes if the type is container")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var tmpl *template.Template
	var err error
	var obj []byte

	if *tmplStr != "" {
		if tmpl, err = template.New("").Funcs(funcMap).Parse(*tmplStr); err != nil {
			return Cli.StatusError{StatusCode: 64,
				Status: "Template parsing error: " + err.Error()}
		}
	}

	if *inspectType != "" && *inspectType != "container" && *inspectType != "image" {
		return fmt.Errorf("%q is not a valid value for --type", *inspectType)
	}

	indented := new(bytes.Buffer)
	indented.WriteString("[\n")
	status := 0
	isImage := false

	v := url.Values{}
	if *size {
		v.Set("size", "1")
	}

	for _, name := range cmd.Args() {
		if *inspectType == "" || *inspectType == "container" {
			obj, _, err = readBody(cli.call("GET", "/containers/"+name+"/json?"+v.Encode(), nil, nil))
			if err != nil {
				if err == errConnectionFailed {
					return err
				}
				if *inspectType == "container" {
					if strings.Contains(err.Error(), "No such") {
						fmt.Fprintf(cli.err, "Error: No such container: %s\n", name)
					} else {
						fmt.Fprintf(cli.err, "%s", err)
					}
					status = 1
					continue
				}
			}
		}

		if obj == nil && (*inspectType == "" || *inspectType == "image") {
			obj, _, err = readBody(cli.call("GET", "/images/"+name+"/json", nil, nil))
			isImage = true
			if err != nil {
				if err == errConnectionFailed {
					return err
				}
				if strings.Contains(err.Error(), "No such") {
					if *inspectType == "" {
						fmt.Fprintf(cli.err, "Error: No such image or container: %s\n", name)
					} else {
						fmt.Fprintf(cli.err, "Error: No such image: %s\n", name)
					}
				} else {
					fmt.Fprintf(cli.err, "%s", err)
				}
				status = 1
				continue
			}
		}

		if tmpl == nil {
			if err := json.Indent(indented, obj, "", "    "); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
		} else {
			rdr := bytes.NewReader(obj)
			dec := json.NewDecoder(rdr)
			buf := bytes.NewBufferString("")

			if isImage {
				inspPtr := types.ImageInspect{}
				if err := dec.Decode(&inspPtr); err != nil {
					fmt.Fprintf(cli.err, "Unable to read inspect data: %v\n", err)
					status = 1
					break
				}
				if err := tmpl.Execute(buf, inspPtr); err != nil {
					rdr.Seek(0, 0)
					var ok bool

					if buf, ok = cli.decodeRawInspect(tmpl, dec); !ok {
						fmt.Fprintf(cli.err, "Template parsing error: %v\n", err)
						status = 1
						break
					}
				}
			} else {
				inspPtr := types.ContainerJSON{}
				if err := dec.Decode(&inspPtr); err != nil {
					fmt.Fprintf(cli.err, "Unable to read inspect data: %v\n", err)
					status = 1
					break
				}
				if err := tmpl.Execute(buf, inspPtr); err != nil {
					rdr.Seek(0, 0)
					var ok bool

					if buf, ok = cli.decodeRawInspect(tmpl, dec); !ok {
						fmt.Fprintf(cli.err, "Template parsing error: %v\n", err)
						status = 1
						break
					}
				}
			}

			cli.out.Write(buf.Bytes())
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
		// Note that we will always write "[]" when "-f" isn't specified,
		// to make sure the output would always be array, see
		// https://github.com/docker/docker/pull/9500#issuecomment-65846734
		if _, err := io.Copy(cli.out, indented); err != nil {
			return err
		}
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// decodeRawInspect executes the inspect template with a raw interface.
// This allows docker cli to parse inspect structs injected with Swarm fields.
// Unfortunately, go 1.4 doesn't fail executing invalid templates when the input is an interface.
// It doesn't allow to modify this behavior either, sending <no value> messages to the output.
// We assume that the template is invalid when there is a <no value>, if the template was valid
// we'd get <nil> or "" values. In that case we fail with the original error raised executing the
// template with the typed input.
//
// TODO: Go 1.5 allows to customize the error behavior, we can probably get rid of this as soon as
// we build Docker with that version:
// https://golang.org/pkg/text/template/#Template.Option
func (cli *DockerCli) decodeRawInspect(tmpl *template.Template, dec *json.Decoder) (*bytes.Buffer, bool) {
	var raw interface{}
	buf := bytes.NewBufferString("")

	if rawErr := dec.Decode(&raw); rawErr != nil {
		fmt.Fprintf(cli.err, "Unable to read inspect data: %v\n", rawErr)
		return buf, false
	}

	if rawErr := tmpl.Execute(buf, raw); rawErr != nil {
		return buf, false
	}

	if strings.Contains(buf.String(), "<no value>") {
		return buf, false
	}
	return buf, true
}
