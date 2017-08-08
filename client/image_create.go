package client

import (
	"encoding/json"
	"io"
	"net/url"

	"golang.org/x/net/context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/system"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageCreate creates a new image based in the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(ctx context.Context, parentReference string, options types.ImageCreateOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(parentReference)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", reference.FamiliarName(ref))
	query.Set("tag", getAPITagFromNamedRef(ref))
	resp, err := cli.tryImageCreate(ctx, query, options.RegistryAuth, options.Platform)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) tryImageCreate(ctx context.Context, query url.Values, registryAuth string, platform specs.Platform) (serverResponse, error) {
	headers := map[string][]string{"X-Registry-Auth": {registryAuth}}

	// TODO @jhowardmsft: system.IsPlatformEmpty is a temporary function. We need to move
	// (in the reasonably short future) to a package which supports all the platform
	// validation such as is proposed in https://github.com/containerd/containerd/pull/1403
	if !system.IsPlatformEmpty(platform) {
		platformJSON, err := json.Marshal(platform)
		if err != nil {
			return serverResponse{}, err
		}
		headers["X-Requested-Platform"] = []string{string(platformJSON[:])}
	}
	return cli.post(ctx, "/images/create", query, nil, headers)
}
