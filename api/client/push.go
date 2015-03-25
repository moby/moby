package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

// CmdPush pushes an image or repository to the registry.
//
// Usage: docker push NAME[:TAG]
func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := cli.Subcmd("push", "NAME[:TAG]", "Push an image or a repository to the registry", true)
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	name := cmd.Arg(0)

	cli.LoadConfigFile()

	remote, tag := parsers.ParseRepositoryTag(name)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(remote)
	if err != nil {
		return err
	}
	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
	// If we're not using a custom registry, we know the restrictions
	// applied to repository names and can warn the user in advance.
	// Custom repositories can have different rules, and we must also
	// allow pushing by image ID.
	if repoInfo.Official {
		username := authConfig.Username
		if username == "" {
			username = "<user>"
		}
		return fmt.Errorf("You cannot push a \"root\" repository. Please rename your repository to <user>/<repo> (ex: %s/%s)", username, repoInfo.LocalName)
	}

	v := url.Values{}
	v.Set("tag", tag)

	push := func(authConfig registry.AuthConfig) error {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		return cli.stream("POST", "/images/"+remote+"/push?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := push(authConfig); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nPlease login prior to push:")
			if err := cli.CmdLogin(repoInfo.Index.GetAuthConfigKey()); err != nil {
				return err
			}
			authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
			return push(authConfig)
		}
		return err
	}
	return nil
}
