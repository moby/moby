package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
)

// ImagePush requests the docker host to push an image to a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// It's up to the caller to handle the io.ReadCloser and close it properly.
func (cli *Client) ImagePush(ctx context.Context, image string, options image.PushOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, err
	}

	if _, isCanonical := ref.(reference.Canonical); isCanonical {
		return nil, errors.New("cannot push a digest reference")
	}

	query := url.Values{}
	if !options.All {
		ref = reference.TagNameOnly(ref)
		if tagged, ok := ref.(reference.Tagged); ok {
			query.Set("tag", tagged.Tag())
		}
	}

	if options.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.46", "platform"); err != nil {
			return nil, err
		}

		p := *options.Platform
		pJson, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("invalid platform: %v", err)
		}

		query.Set("platform", string(pJson))
	}

	// PrivilegeFunc was added in [18472] as an alternative to passing static
	// authentication. The default was still to try the static authentication
	// before calling the PrivilegeFunc (if present).
	//
	// For now, we need to keep this behavior, as PrivilegeFunc may be an
	// interactive prompt, however, we should change this to only use static
	// auth if not empty. Ultimately, we should deprecate its use in favor of
	// callers providing a PrivilegeFunc (which can be chained), or a list of
	// PrivilegeFuncs.
	//
	// [18472]: https://github.com/moby/moby/commit/e78f02c4dbc3cada909c114fef6b6643969ab912
	resp, err := cli.tryImagePush(ctx, ref.Name(), query, ChainPrivilegeFuncs(staticAuth(options.RegistryAuth), options.PrivilegeFunc))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cli *Client) tryImagePush(ctx context.Context, imageID string, query url.Values, resolveAuth registry.RequestAuthConfig) (*http.Response, error) {
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
		resp, err := cli.post(ctx, "/images/"+imageID+"/push", query, nil, hdr)
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
