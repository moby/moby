package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/utils"
)

func (cli *DockerCli) CmdVolume(args ...string) error {
	description := "Manage Docker Volumes\n\nCommands:\n"
	commands := [][]string{
		{"ls", "List volumes"},
		{"inspect", "Inspect a volume"},
		{"create", "Create a volume"},
		{"rm", "Remove a volume"},
	}

	for _, cmd := range commands {
		description += fmt.Sprintf("    %-10.10s%s\n", cmd[0], cmd[1])
	}

	description += "\nRun 'docker volume COMMNAD --help' for more information on a command."

	cmd := cli.Subcmd("volume", "[COMMAND]", description, true)
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		cmd.Usage()
		return nil
	}

	return cli.CmdVolumeLs(args...)
}

func (cli *DockerCli) CmdVolumeLs(args ...string) error {
	cmd := cli.Subcmd("volume ls", "", "List volumes", true)

	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display volume names")
	size := cmd.Bool([]string{"s", "-size"}, false, "Display total size of volumes")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Provide filter values (i.e. 'dangling=true')")
	cmd.Parse(args)

	volFilterArgs := filters.Args{}
	for _, f := range flFilter.GetAll() {
		var err error
		volFilterArgs, err = filters.ParseFlag(f, volFilterArgs)
		if err != nil {
			return err
		}
	}

	v := url.Values{}
	if *quiet {
		v.Set("quiet", "1")
	}

	if *size {
		v.Set("size", "1")
	}

	if len(volFilterArgs) > 0 {
		filterJson, err := filters.ToParam(volFilterArgs)
		if err != nil {
			return err
		}
		v.Set("filters", filterJson)
	}

	body, _, err := readBody(cli.call("GET", "/volumes?"+v.Encode(), nil, false))
	if err != nil {
		return err
	}

	outs := engine.NewTable("Created", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprint(w, "NAME\tCREATED\tUSED COUNT\t")
		if *size {
			fmt.Fprintf(w, "SIZE")
		}
	}
	fmt.Fprint(w, "\n")

	for _, out := range outs.Data {
		var (
			name      = out.Get("Name")
			usedCount = out.Get("Count")
			created   = units.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0)))
		)

		if *quiet {
			fmt.Fprintln(w, name)
			continue
		}
		fmt.Fprintf(w, name+"\t")
		fmt.Fprintf(w, fmt.Sprintf("%s ago\t", created))
		fmt.Fprintf(w, usedCount+"\t")

		if *size {
			fmt.Fprintf(w, fmt.Sprintf("%s\t", units.HumanSize(float64(out.GetInt64("Size")))))
		}
		fmt.Fprintf(w, "\n")
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func (cli *DockerCli) CmdVolumeInspect(args ...string) error {
	cmd := cli.Subcmd("volume inspect", "", "Inspect a volume", true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template.")
	size := cmd.Bool([]string{"s", "-size"}, false, "Show the size of the volume in output")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

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
		v := url.Values{}
		if *size {
			v.Set("size", "1")
		}
		obj, _, err := readBody(cli.call("GET", "/volumes/"+name+"?"+v.Encode(), nil, false))
		if err != nil {
			if strings.Contains(err.Error(), "No such") {
				fmt.Fprintf(cli.err, "Error: No such volume: %s\n", name)
			} else {
				fmt.Fprintf(cli.err, "%s", err)
			}
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
	indented.WriteByte(']')

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

func (cli *DockerCli) CmdVolumeRm(args ...string) error {
	cmd := cli.Subcmd("volume rm", "", "remove a volume", true)
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("DELETE", "/volumes/"+name, nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more volumes")
			continue
		}
		fmt.Fprintf(cli.out, "%s\n", name)
	}
	return encounteredError
}

func (cli *DockerCli) CmdVolumeCreate(args ...string) error {
	var (
		cmd  = cli.Subcmd("volume create", "", "create a volume", true)
		path = cmd.String([]string{"p", "-path"}, "", "Specify path of new volume")
		mode = cmd.String([]string{"m", "-mode"}, "rw", "Sepcify write-mode of volume")
		name = cmd.String([]string{"n", "-name"}, "", "Specify name of volume")
	)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	v := map[string]string{
		"mode": *mode,
		"path": *path,
		"name": *name,
	}

	stream, _, err := cli.call("POST", "/volumes", v, false)
	if err != nil {
		return err
	}

	var result engine.Env
	if err := result.Decode(stream); err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", result.Get("Name"))
	return nil
}
