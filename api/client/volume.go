package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"text/tabwriter"
	"text/template"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers/filters"
)

// CmdVolume is the parent subcommand for all volume commands
//
// Usage: docker volume <COMMAND> <OPTS>
func (cli *DockerCli) CmdVolume(args ...string) error {
	description := Cli.DockerCommands["volume"].Description + "\n\nCommands:\n"
	commands := [][]string{
		{"create", "Create a volume"},
		{"inspect", "Return low-level information on a volume"},
		{"ls", "List volumes"},
		{"rm", "Remove a volume"},
	}

	for _, cmd := range commands {
		description += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	description += "\nRun 'docker volume COMMAND --help' for more information on a command"
	cmd := Cli.Subcmd("volume", []string{"[COMMAND]"}, description, false)

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdVolumeLs outputs a list of Docker volumes.
//
// Usage: docker volume ls [OPTIONS]
func (cli *DockerCli) CmdVolumeLs(args ...string) error {
	cmd := Cli.Subcmd("volume ls", nil, "List volumes", true)

	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display volume names")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Provide filter values (i.e. 'dangling=true')")

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	volFilterArgs := filters.Args{}
	for _, f := range flFilter.GetAll() {
		var err error
		volFilterArgs, err = filters.ParseFlag(f, volFilterArgs)
		if err != nil {
			return err
		}
	}

	v := url.Values{}
	if len(volFilterArgs) > 0 {
		filterJSON, err := filters.ToParam(volFilterArgs)
		if err != nil {
			return err
		}
		v.Set("filters", filterJSON)
	}

	resp, err := cli.call("GET", "/volumes?"+v.Encode(), nil, nil)
	if err != nil {
		return err
	}

	var volumes types.VolumesListResponse
	if err := json.NewDecoder(resp.body).Decode(&volumes); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintf(w, "DRIVER \tVOLUME NAME")
		fmt.Fprintf(w, "\n")
	}

	for _, vol := range volumes.Volumes {
		if *quiet {
			fmt.Fprintln(w, vol.Name)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", vol.Driver, vol.Name)
	}
	w.Flush()
	return nil
}

// CmdVolumeInspect displays low-level information on one or more volumes.
//
// Usage: docker volume inspect [OPTIONS] VOLUME [VOLUME...]
func (cli *DockerCli) CmdVolumeInspect(args ...string) error {
	cmd := Cli.Subcmd("volume inspect", []string{"VOLUME [VOLUME...]"}, "Return low-level information on a volume", true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "Format the output using the given go template")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	var tmpl *template.Template
	if *tmplStr != "" {
		var err error
		tmpl, err = template.New("").Funcs(funcMap).Parse(*tmplStr)
		if err != nil {
			return err
		}
	}

	var status = 0
	var volumes []*types.Volume
	for _, name := range cmd.Args() {
		resp, err := cli.call("GET", "/volumes/"+name, nil, nil)
		if err != nil {
			return err
		}

		var volume types.Volume
		if err := json.NewDecoder(resp.body).Decode(&volume); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}

		if tmpl == nil {
			volumes = append(volumes, &volume)
			continue
		}

		if err := tmpl.Execute(cli.out, &volume); err != nil {
			if err := tmpl.Execute(cli.out, &volume); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
		}
		io.WriteString(cli.out, "\n")
	}

	if tmpl != nil {
		return nil
	}

	b, err := json.MarshalIndent(volumes, "", "    ")
	if err != nil {
		return err
	}
	_, err = io.Copy(cli.out, bytes.NewReader(b))
	if err != nil {
		return err
	}
	io.WriteString(cli.out, "\n")

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// CmdVolumeCreate creates a new container from a given image.
//
// Usage: docker volume create [OPTIONS]
func (cli *DockerCli) CmdVolumeCreate(args ...string) error {
	cmd := Cli.Subcmd("volume create", nil, "Create a volume", true)
	flDriver := cmd.String([]string{"d", "-driver"}, "local", "Specify volume driver name")
	flName := cmd.String([]string{"-name"}, "", "Specify volume name")

	flDriverOpts := opts.NewMapOpts(nil, nil)
	cmd.Var(flDriverOpts, []string{"o", "-opt"}, "Set driver specific options")

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	volReq := &types.VolumeCreateRequest{
		Driver:     *flDriver,
		DriverOpts: flDriverOpts.GetAll(),
	}

	if *flName != "" {
		volReq.Name = *flName
	}

	resp, err := cli.call("POST", "/volumes/create", volReq, nil)
	if err != nil {
		return err
	}

	var vol types.Volume
	if err := json.NewDecoder(resp.body).Decode(&vol); err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", vol.Name)
	return nil
}

// CmdVolumeRm removes one or more containers.
//
// Usage: docker volume rm VOLUME [VOLUME...]
func (cli *DockerCli) CmdVolumeRm(args ...string) error {
	cmd := Cli.Subcmd("volume rm", []string{"VOLUME [VOLUME...]"}, "Remove a volume", true)
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	var status = 0
	for _, name := range cmd.Args() {
		_, err := cli.call("DELETE", "/volumes/"+name, nil, nil)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(cli.out, "%s\n", name)
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}
