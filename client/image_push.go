package client // import "github.com/docker/docker/client"

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
)

// ImagePush requests the docker host to push an image to a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// It's up to the caller to handle the io.ReadCloser and close it properly.
func (cli *Client) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, err
	}

	if _, isCanonical := ref.(reference.Canonical); isCanonical {
		return nil, errors.New("cannot push a digest reference")
	}

	name := reference.FamiliarName(ref)
	query := url.Values{}
	if !options.All {
		ref = reference.TagNameOnly(ref)
		if tagged, ok := ref.(reference.Tagged); ok {
			query.Set("tag", tagged.Tag())
		}
	}

	resp, err := cli.tryImagePush(ctx, name, query, options.RegistryAuth)
	if errdefs.IsUnauthorized(err) && options.PrivilegeFunc != nil {
		newAuthHeader, privilegeErr := options.PrivilegeFunc()
		if privilegeErr != nil {
			return nil, privilegeErr
		}
		resp, err = cli.tryImagePush(ctx, name, query, newAuthHeader)
	}
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) tryImagePush(ctx context.Context, imageID string, query url.Values, registryAuth string) (serverResponse, error) {
	// Always send a body (which may be an empty JSON document ("{}")) to prevent
	// EOF errors on older daemons which had faulty fallback code for handling
	// authentication in the body when no auth-header was set, resulting in;
	//
	//	Error response from daemon: bad parameters and missing X-Registry-Auth: invalid X-Registry-Auth header: EOF
	//
	// We use [http.NoBody], which gets marshaled to an empty JSON document.
	//
	// see: https://github.com/moby/moby/commit/ea29dffaa541289591aa44fa85d2a596ce860e16
	return cli.post(ctx, "/images/"+imageID+"/push", query, http.NoBody, http.Header{
		registry.AuthHeader: {registryAuth},
	})
}
