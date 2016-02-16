package client

import (
	"errors"
	"io"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/jsonmessage"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
)

// CmdPush pushes an image or repository to the registry.
//
// Usage: docker push NAME[:TAG]
func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := Cli.Subcmd("push", []string{"NAME[:TAG]"}, Cli.DockerCommands["push"].Description, true)
	addTrustedFlags(cmd, false)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	ref, err := reference.ParseNamed(cmd.Arg(0))
	if err != nil {
		return err
	}

	var tag string
	switch x := ref.(type) {
	case reference.Canonical:
		return errors.New("cannot push a digest reference")
	case reference.NamedTagged:
		tag = x.Tag()
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}
	// Resolve the Auth config relevant for this server
	authConfig := cli.resolveAuthConfig(cli.configFile.AuthConfigs, repoInfo.Index)

	requestPrivilege := cli.registryAuthenticationPrivilegedFunc(repoInfo.Index, "push")
	if isTrusted() {
		return cli.trustedPush(repoInfo, tag, authConfig, requestPrivilege)
	}

	responseBody, err := cli.imagePushPrivileged(authConfig, ref.Name(), tag, requestPrivilege)
	if err != nil {
		return err
	}

	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

func (cli *DockerCli) imagePushPrivileged(authConfig types.AuthConfig, imageID, tag string, requestPrivilege client.RequestPrivilegeFunc) (io.ReadCloser, error) {
	encodedAuth, err := encodeAuthToBase64(authConfig)
	if err != nil {
		return nil, err
	}
	options := types.ImagePushOptions{
		ImageID:      imageID,
		Tag:          tag,
		RegistryAuth: encodedAuth,
	}

	return cli.client.ImagePush(context.Background(), options, requestPrivilege)
}
