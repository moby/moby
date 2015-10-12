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

// CmdPull pulls an image or a repository from the registry.
//
// Usage: docker pull [OPTIONS] IMAGENAME[:TAG|@DIGEST]
func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := Cli.Subcmd("pull", []string{"NAME[:TAG|@DIGEST]"}, Cli.DockerCommands["pull"].Description, true)
	allTags := cmd.Bool([]string{"a", "-all-tags"}, false, "Download all tagged images in the repository")
	addTrustedFlags(cmd, true)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)
	remote := cmd.Arg(0)

	taglessRemote, tag := parsers.ParseRepositoryTag(remote)
	if tag == "" && !*allTags {
		tag = tags.DefaultTag
		fmt.Fprintf(cli.out, "Using default tag: %s\n", tag)
	} else if tag != "" && *allTags {
		return fmt.Errorf("tag can't be used with --all-tags/-a")
	}

	ref := registry.ParseReference(tag)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(taglessRemote)
	if err != nil {
		return err
	}

	if isTrusted() && !ref.HasDigest() {
		// Check if tag is digest
		authConfig := registry.ResolveAuthConfig(cli.configFile, repoInfo.Index)
		return cli.trustedPull(repoInfo, ref, authConfig)
	}

	v := url.Values{}
	v.Set("fromImage", ref.ImageName(taglessRemote))

	_, _, err = cli.clientRequestAttemptLogin("POST", "/images/create?"+v.Encode(), nil, cli.out, repoInfo.Index, "pull")
	return err
}
