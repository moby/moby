package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/docker/docker/graph"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

// CmdPull pulls an image or a repository from the registry.
//
// Usage: docker pull [OPTIONS] IMAGENAME[:TAG|@DIGEST]
func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := cli.Subcmd("pull", "NAME[:TAG|@DIGEST]", "Pull an image or a repository from the registry", true)
	allTags := cmd.Bool([]string{"a", "-all-tags"}, false, "Download all tagged images in the repository")
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	var (
		v         = url.Values{}
		remote    = cmd.Arg(0)
		newRemote = remote
	)
	taglessRemote, tag := parsers.ParseRepositoryTag(remote)
	if tag == "" && !*allTags {
		newRemote = utils.ImageReference(taglessRemote, graph.DEFAULTTAG)
	}
	if tag != "" && *allTags {
		return fmt.Errorf("tag can't be used with --all-tags/-a")
	}

	v.Set("fromImage", newRemote)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(taglessRemote)
	if err != nil {
		return err
	}

	cli.LoadConfigFile()

	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)

	pull := func(authConfig registry.AuthConfig) error {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		return cli.stream("POST", "/images/create?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := pull(authConfig); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nPlease login prior to pull:")
			if err := cli.CmdLogin(repoInfo.Index.GetAuthConfigKey()); err != nil {
				return err
			}
			authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
			return pull(authConfig)
		}
		return err
	}

	return nil
}
