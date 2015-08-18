package client

import (
	"fmt"
	"net/url"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

// CmdPush pushes one or more images or repositories to the registry.
//
// Usage: docker push NAME[:TAG]
func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := Cli.Subcmd("push", []string{"NAME[:TAG] [NAME[:TAG]...]"}, "Push one or more images or repositories to a registry", true)
	addTrustedFlags(cmd, false)
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errNames []string

	// Grouping tags by remote (if any)
	repositories := make(map[string][]string)

	for _, arg := range cmd.Args() {
		remote, tag := parsers.ParseRepositoryTag(arg)
		repositories[remote] = append(repositories[remote], tag)
	}

	for remote, tags := range repositories {

		// Resolve the Repository name from fqn to RepositoryInfo
		repoInfo, err := registry.ParseRepositoryInfo(remote)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			errNames = append(errNames, fmt.Sprintf("%s:%s", remote, tags))
			continue
		}
		// Resolve the Auth config relevant for this server
		authConfig := registry.ResolveAuthConfig(cli.configFile, repoInfo.Index)
		// If we're not using a custom registry, we know the restrictions
		// applied to repository names and can warn the user in advance.
		// Custom repositories can have different rules, and we must also
		// allow pushing by image ID.
		if repoInfo.Official {
			username := authConfig.Username
			if username == "" {
				username = "<user>"
			}
			fmt.Fprintf(cli.err, "You cannot push a \"root\" repository. Please rename your repository to <user>/<repo> (ex: %s/%s)\n", username, repoInfo.LocalName)
			errNames = append(errNames, fmt.Sprintf("%s:%s", remote, tags))
			continue
		}

		if isTrusted() {
			return cli.trustedPush(repoInfo, tags, authConfig)
		}

		v := url.Values{}
		for _, tag := range tags {
			v.Add("tag", tag)
		}

		_, _, err = cli.clientRequestAttemptLogin("POST", "/images/"+remote+"/push?"+v.Encode(), nil, cli.out, repoInfo.Index, "push")
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			errNames = append(errNames, fmt.Sprintf("%s:%s", remote, tags))
		} else {
			fmt.Fprint(cli.out, "\n")
		}
	}
	if len(errNames) > 0 {
		return fmt.Errorf("Error: failed to push images: %v", errNames)
	}
	return nil
}
