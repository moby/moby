package client

import (
	"io"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/types"
)

// ImagePushPrivileged push the image
func (cli *DockerCli) ImagePushPrivileged(ctx context.Context, authConfig types.AuthConfig, ref string, requestPrivilege types.RequestPrivilegeFunc) (io.ReadCloser, error) {
	encodedAuth, err := EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, err
	}
	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	return cli.client.ImagePush(ctx, ref, options)
}
