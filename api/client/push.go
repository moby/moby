package client

import (
	"errors"
	"io"

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

	requestPrivilege := cli.registryAuthenticationPrivilegedFunc(repoInfo.Index, "push", reference.IsReferenceFullyQualified(ref))

	if isTrusted() {
		return cli.trustedPush(ref, repoInfo, tag, requestPrivilege)
	}

	responseBody, err := cli.imagePushPrivileged(ref, tag, requestPrivilege)
	if err != nil {
		return err
	}

	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

func (cli *DockerCli) imagePushPrivileged(ref reference.Named, tag string, requestPrivilege client.RequestPrivilegeFunc) (io.ReadCloser, error) {
	encodedAuth, err := cli.getEncodedAuth(ref)
	if err != nil {
		return nil, err
	}
	options := types.ImagePushOptions{
		ImageID:      ref.Name(),
		Tag:          tag,
		RegistryAuth: encodedAuth,
	}

	return cli.client.ImagePush(options, requestPrivilege)
}
