package client

import (
	"context"
	"net/http"
	"net/url"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/registry"
)

// ImageCreate creates a new image based on the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(ctx context.Context, parentReference string, options ImageCreateOptions) (ImageCreateResult, error) {
	ref, err := reference.ParseNormalizedNamed(parentReference)
	if err != nil {
		return ImageCreateResult{}, err
	}

	query := url.Values{}
	query.Set("fromImage", ref.Name())
	query.Set("tag", getAPITagFromNamedRef(ref))
	if len(options.Platforms) > 0 {
		if len(options.Platforms) > 1 {
			// TODO(thaJeztah): update API spec and add equivalent check on the daemon. We need this still for older daemons, which would ignore it.
			return ImageCreateResult{}, cerrdefs.ErrInvalidArgument.WithMessage("specifying multiple platforms is not yet supported")
		}
		query.Set("platform", formatPlatform(options.Platforms[0]))
	}
	resp, err := cli.tryImageCreate(ctx, query, staticAuth(options.RegistryAuth))
	if err != nil {
		return ImageCreateResult{}, err
	}
	return ImageCreateResult{Body: resp.Body}, nil
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
