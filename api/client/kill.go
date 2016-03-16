package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdKill kills one or more running container using SIGKILL or a specified signal.
//
// Usage: docker kill [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdKill(args ...string) error {
	cmd := Cli.Subcmd("kill", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["kill"].Description, true)
	signal := cmd.String([]string{"s", "-signal"}, "KILL", "Signal to send to the container")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errs []string
	for _, name := range cmd.Args() {
		if err := cli.client.ContainerKill(context.Background(), name, *signal); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
