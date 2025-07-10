package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
)

// ImageCreate creates a new image based on the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(ctx context.Context, parentReference string, options image.CreateOptions) (io.ReadCloser, error) {
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
	var lastErr error
	for {
		registryAuth, tryNext, err := getAuth(ctx, resolveAuth)
		if err != nil {
			// TODO(thaJeztah): should we return an "unauthorised error" here to allow the caller to try other options?
			if errors.Is(err, errNoMorePrivilegeFuncs) && lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}
		if registryAuth != "" {
			hdr.Set(registry.AuthHeader, registryAuth)
		}
		resp, err := cli.post(ctx, "/images/create", query, nil, hdr)
		if err == nil {
			// Discard previous errors
			return resp, nil
		} else {
			if IsErrConnectionFailed(err) {
				// Don't retry if we failed to connect to the API.
				return nil, err
			}

			// TODO(thaJeztah); only retry with "IsUnauthorized" and/or "rate limit (StatusTooManyRequests)" errors?
			if !cerrdefs.IsUnauthorized(err) {
				return nil, err
			}

			lastErr = err
		}
		if !tryNext {
			return nil, lastErr
		}
	}
}
