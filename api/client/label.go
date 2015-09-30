package client

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdLabel is the parent subcommand for all label commands
//
// Usage: docker label <COMMAND> <OPTS>
func (cli *DockerCli) CmdLabel(args ...string) error {
	description := "Manage Docker daemon labels\n\nCommands:\n"
	commands := [][]string{
		{"list", "Show the docker daemon labels."},
		{"add", "add new labels to docker daemon."},
		{"remove", "remove any exist labels from docker daemon."},
	}

	for _, cmd := range commands {
		description += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	description += "\nRun 'docker label COMMAND --help' for more information on a command"
	cmd := Cli.Subcmd("label", []string{"[COMMAND]"}, description, true)
	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	return cli.CmdLabelList(args...)
}

// CmdLabelList outputs a list of Docker daemon labels.
//
// Usage: docker label list [OPTIONS]
func (cli *DockerCli) CmdLabelList(args ...string) error {
	cmd := Cli.Subcmd("label list", nil, "List daemon labels", true)

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	resp, err := cli.call("GET", "/labels", nil, nil)
	if err != nil {
		return err
	}

	var labels []string
	if err := json.NewDecoder(resp.body).Decode(&labels); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "Labels:")
	fmt.Fprintf(w, "\n")

	for _, label := range labels {
		fmt.Fprintf(w, "%s\n", label)
	}
	w.Flush()
	return nil
}

// CmdLabelAdd adds a new label to a daemon label.
//
// Usage: docker label add LABEL[LABEL...]
func (cli *DockerCli) CmdLabelAdd(args ...string) error {
	cmd := Cli.Subcmd("label add", nil, "Add a label", true)

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	var status = 0
	for _, name := range cmd.Args() {
		_, err := cli.call("POST", "/labels/"+name, nil, nil)
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

// CmdLabelRemove removes one or more labels.
//
// Usage: docker label remove LABEL[LABEL...]
func (cli *DockerCli) CmdLabelRemove(args ...string) error {
	cmd := Cli.Subcmd("label remove", []string{"LABEL [LABEL...]"}, "Remove a label", true)
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	var status = 0
	for _, name := range cmd.Args() {
		_, err := cli.call("DELETE", "/labels/"+name, nil, nil)
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
