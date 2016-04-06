package client

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdTop displays the running processes of a container.
//
// Usage: docker top CONTAINER
func (cli *DockerCli) CmdTop(args ...string) error {
	cmd := Cli.Subcmd("top", []string{"CONTAINER [ps OPTIONS]"}, Cli.DockerCommands["top"].Description, true)
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var arguments []string
	if cmd.NArg() > 1 {
		arguments = cmd.Args()[1:]
	}

	procList, err := cli.client.ContainerTop(context.Background(), cmd.Arg(0), arguments)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(procList.Titles, "\t"))

	for _, proc := range procList.Processes {
		fmt.Fprintln(w, strings.Join(proc, "\t"))
	}
	w.Flush()
	return nil
}
