package client

import (
	"fmt"
	"os"

	flag "github.com/docker/docker/pkg/mflag"
)

func (cli *DockerCli) CmdHelp(args ...string) error {
	if len(args) > 1 {
		method, exists := cli.getMethod(args[:2]...)
		if exists {
			method("--help")
			return nil
		}
	}
	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Fprintf(cli.err, "docker: '%s' is not a docker command. See 'docker --help'.\n", args[0])
			os.Exit(1)
		} else {
			method("--help")
			return nil
		}
	}

	flag.Usage()

	return nil
}
