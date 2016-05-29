package client

import (
	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdTag tags an image into a repository.
//
// Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]
func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := Cli.Subcmd("tag", []string{"IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]"}, Cli.DockerCommands["tag"].Description, true)
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	return cli.client.ImageTag(context.Background(), cmd.Arg(0), cmd.Arg(1))
}
