package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/url"
	"strings"

	reference "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
)

// ImageCreate creates a new image based on the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(ctx context.Context, parentReference string, options types.ImageCreateOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(parentReference)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", reference.FamiliarName(ref))
	query.Set("tag", getAPITagFromNamedRef(ref))
	if options.Platform != "" {
		query.Set("platform", strings.ToLower(options.Platform))
	}
	resp, err := cli.tryImageCreate(ctx, query, options.RegistryAuth)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) tryImageCreate(ctx context.Context, query url.Values, registryAuth string) (serverResponse, error) {
	headers := map[string][]string{registry.AuthHeader: {registryAuth}}
	return cli.post(ctx, "/images/create", query, nil, headers)
}
