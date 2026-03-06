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
	if options.ChallengeHandlerFunc != nil {
		query.Set("clientAuth", "1")
	}

	resp, err := cli.tryImagePush(ctx, ref.Name(), query, staticAuth(options.RegistryAuth))
	if challenge := resp.Header.Get("WWW-Authenticate"); challenge != "" && err != nil {
		if options.ChallengeHandlerFunc != nil {
			var newAuthHeader string
			newAuthHeader, err = options.ChallengeHandlerFunc(ctx, challenge)
			if err != nil {
				return nil, err
			}
			resp, err = cli.tryImagePush(ctx, ref.Name(), query, staticAuth(newAuthHeader))
		}
	}
	if cerrdefs.IsUnauthorized(err) && options.PrivilegeFunc != nil {
		resp, err = cli.tryImagePush(ctx, ref.Name(), query, options.PrivilegeFunc)
	}
	if err != nil {
		return nil, err
	}
	return internal.NewJSONMessageStream(resp.Body), nil
}

func (cli *Client) tryImagePush(ctx context.Context, imageID string, query url.Values, resolveAuth registry.RequestAuthConfig) (*http.Response, error) {
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

	// Always send a body (which may be an empty JSON document ("{}")) to prevent
	// EOF errors on older daemons which had faulty fallback code for handling
	// authentication in the body when no auth-header was set, resulting in;
	//
	//	Error response from daemon: bad parameters and missing X-Registry-Auth: invalid X-Registry-Auth header: EOF
	//
	// We use [http.NoBody], which gets marshaled to an empty JSON document.
	//
	// see: https://github.com/moby/moby/commit/ea29dffaa541289591aa44fa85d2a596ce860e16
	return cli.post(ctx, "/images/"+imageID+"/push", query, http.NoBody, hdr)
}
