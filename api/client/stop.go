package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdStop stops one or more containers.
//
// A running container is stopped by first sending SIGTERM and then SIGKILL if the container fails to stop within a grace period (the default is 10 seconds).
//
// Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStop(args ...string) error {
	cmd := Cli.Subcmd("stop", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["stop"].Description+".\nSending SIGTERM and then SIGKILL after a grace period", true)
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "Seconds to wait for stop before killing it")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errs []string
	for _, name := range cmd.Args() {
		if err := cli.client.ContainerStop(context.Background(), name, *nSeconds); err != nil {
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
