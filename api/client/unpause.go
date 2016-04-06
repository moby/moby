package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdUnpause unpauses all processes within a container, for one or more containers.
//
// Usage: docker unpause CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdUnpause(args ...string) error {
	cmd := Cli.Subcmd("unpause", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["unpause"].Description, true)
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errs []string
	for _, name := range cmd.Args() {
		if err := cli.client.ContainerUnpause(context.Background(), name); err != nil {
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
