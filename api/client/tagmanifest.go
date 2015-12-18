package client

import (
	"net/url"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

// CmdTag tags a manifest on a remote registry.
//
// Usage: docker tagmani [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]
func (cli *DockerCli) CmdTagManifest(args ...string) error {
	cmd := Cli.Subcmd("tagmani", []string{"IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]"}, Cli.DockerCommands["tagmani"].Description, true)
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	var (
		repository, tag = parsers.ParseRepositoryTag(cmd.Arg(1))
		v               = url.Values{}
	)

	//Check if the given image name can be resolved
	if err := registry.ValidateRepositoryName(repository); err != nil {
		return err
	}
	v.Set("repo", repository)
	v.Set("tag", tag)

	if _, _, err := readBody(cli.call("POST", "/images/"+cmd.Arg(0)+"/tagmanifest?"+v.Encode(), nil, nil)); err != nil {
		return err
	}
	return nil
}
