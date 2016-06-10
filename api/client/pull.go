package client

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/engine-api/types"
)

// ImagePullPrivileged pulls the image and displays it to the output
func (cli *DockerCli) ImagePullPrivileged(ctx context.Context, authConfig types.AuthConfig, ref string, requestPrivilege types.RequestPrivilegeFunc, all bool) error {

	encodedAuth, err := EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}
	options := types.ImagePullOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
		All:           all,
	}

	responseBody, err := cli.client.ImagePull(ctx, ref, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut, nil)
}
