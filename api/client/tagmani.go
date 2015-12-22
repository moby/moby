package client

import (
	"fmt"
	"net/url"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/graph/tags"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

// CmdTag tags a manifest on a remote registry.
//
// Usage: docker tagmani [OPTIONS] [REGISTRYHOST/][USERNAME/]IMAGENAME[:TAG|@DIGEST] [TAG]
func (cli *DockerCli) CmdTagmani(args ...string) error {
	cmd := Cli.Subcmd("tagmani", []string{"[REGISTRYHOST/][USERNAME/]NAME[:TAG] [TAG]"}, Cli.DockerCommands["tagmani"].Description, true)
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)
	remote := cmd.Arg(0)
	newTag := cmd.Arg(1)

	taglessRemote, tag := parsers.ParseRepositoryTag(remote)
	if tag == "" {
		tag = tags.DefaultTag
		fmt.Fprintf(cli.out, "Using default tag: %s\n", tag)
	}

	ref := registry.ParseReference(tag)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(taglessRemote)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("fromImage", ref.ImageName(taglessRemote))
	v.Set("newTag", newTag)

	if _, _, err := cli.clientRequestAttemptLogin("POST", "/images/"+cmd.Arg(0)+"/tagmanifest?"+v.Encode(), nil, cli.out, repoInfo.Index, "pull"); err != nil {
		return err
	}
	return nil
}
