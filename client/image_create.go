package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/registry"
)

// ImageCreate creates a new image based on the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(ctx context.Context, parentReference string, options ImageCreateOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(parentReference)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", ref.Name())
	query.Set("tag", getAPITagFromNamedRef(ref))
	if options.Platform != "" {
		query.Set("platform", strings.ToLower(options.Platform))
	}
	resp, err := cli.tryImageCreate(ctx, query, staticAuth(options.RegistryAuth))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cli *Client) tryImageCreate(ctx context.Context, query url.Values, resolveAuth registry.RequestAuthConfig) (*http.Response, error) {
	hdr := http.Header{}
	if resolveAuth != nil {
		registryAuth, err := resolveAuth(ctx)
		if err != nil {
			return nil, err
		}
		if registryAuth != "" {
			hdr.Set(registry.AuthHeader, registryAuth)
		}
	}
	return cli.post(ctx, "/images/create", query, nil, hdr)
}
