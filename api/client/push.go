package client

import (
	"net/url"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

// CmdPush pushes an image or repository to the registry.
//
// Usage: docker push NAME[:TAG]
func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := Cli.Subcmd("push", []string{"NAME[:TAG]"}, Cli.DockerCommands["push"].Description, true)
	addTrustedFlags(cmd, false)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	remote, tag := parsers.ParseRepositoryTag(cmd.Arg(0))

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(remote)
	if err != nil {
		return err
	}

	// Resolve the Auth config relevant for this server
	authConfig := registry.ResolveAuthConfig(cli.configFile, repoInfo.Index)

	if isTrusted() {
		return cli.trustedPush(repoInfo, tag, authConfig)
	}

	v := url.Values{}
	v.Set("tag", tag)

	_, _, err = cli.clientRequestAttemptLogin("POST", "/images/"+remote+"/push?"+v.Encode(), nil, cli.out, repoInfo.Index, "push")
	return err
}
