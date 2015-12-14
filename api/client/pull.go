package client

import (
	"errors"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/client/lib"
	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/jsonmessage"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
	tagpkg "github.com/docker/docker/tag"
)

var errTagCantBeUsed = errors.New("tag can't be used with --all-tags/-a")

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

	distributionRef, err := reference.ParseNamed(remote)
	if err != nil {
		return err
	}

	var tag string
	switch x := distributionRef.(type) {
	case reference.Digested:
		if *allTags {
			return errTagCantBeUsed
		}
		tag = x.Digest().String()
	case reference.Tagged:
		if *allTags {
			return errTagCantBeUsed
		}
		tag = x.Tag()
	default:
		if !*allTags {
			tag = tagpkg.DefaultTag
			distributionRef, err = reference.WithTag(distributionRef, tag)
			if err != nil {
				return err
			}
			fmt.Fprintf(cli.out, "Using default tag: %s\n", tag)
		}
	}

	ref := registry.ParseReference(tag)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(distributionRef)
	if err != nil {
		return err
	}

	authConfig := registry.ResolveAuthConfig(cli.configFile.AuthConfigs, repoInfo.Index)
	requestPrivilege := cli.registryAuthenticationPrivilegedFunc(repoInfo.Index, "pull")

	if isTrusted() && !ref.HasDigest() {
		// Check if tag is digest
		return cli.trustedPull(repoInfo, ref, authConfig, requestPrivilege)
	}

	return cli.imagePullPrivileged(authConfig, distributionRef.String(), "", requestPrivilege)
}

func (cli *DockerCli) imagePullPrivileged(authConfig cliconfig.AuthConfig, imageID, tag string, requestPrivilege lib.RequestPrivilegeFunc) error {

	encodedAuth, err := authConfig.EncodeToBase64()
	if err != nil {
		return err
	}
	options := types.ImagePullOptions{
		ImageID:      imageID,
		Tag:          tag,
		RegistryAuth: encodedAuth,
	}

	responseBody, err := cli.client.ImagePull(options, requestPrivilege)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut)
}
