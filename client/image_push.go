package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client/internal"
)

type ImagePushResponse interface {
	io.ReadCloser
	JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error]
	Wait(ctx context.Context) error
}

// ImagePush requests the docker host to push an image to a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// Callers can
//   - use [ImagePushResponse.Wait] to wait for push to complete
//   - use [ImagePushResponse.JSONMessages] to monitor pull progress as a sequence
//     of JSONMessages, [ImagePushResponse.Close] does not need to be called in this case.
//   - use the [io.Reader] interface and call [ImagePushResponse.Close] after processing.
func (cli *Client) ImagePush(ctx context.Context, image string, options ImagePushOptions) (ImagePushResponse, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, err
	}

	if _, ok := ref.(reference.Digested); ok {
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
		if err := cli.requiresVersion(ctx, "1.46", "platform"); err != nil {
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
	return internal.NewJSONMessageStream(resp.Body), nil
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

		// Always send a body (which may be an empty JSON document ("{}")) to prevent
		// EOF errors on older daemons which had faulty fallback code for handling
		// authentication in the body when no auth-header was set, resulting in;
		//
		//	Error response from daemon: bad parameters and missing X-Registry-Auth: invalid X-Registry-Auth header: EOF
		//
		// We use [http.NoBody], which gets marshaled to an empty JSON document.
		//
		// see: https://github.com/moby/moby/commit/ea29dffaa541289591aa44fa85d2a596ce860e16
		resp, err := cli.post(ctx, "/images/"+imageID+"/push", query, http.NoBody, hdr)
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
