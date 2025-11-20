package client

import (
	"context"
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

type ImagePullResponse interface {
	io.ReadCloser
	JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error]
	Wait(ctx context.Context) error
}

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// Callers can:
//   - use [ImagePullResponse.Wait] to wait for pull to complete
//   - use [ImagePullResponse.JSONMessages] to monitor pull progress as a sequence
//     of JSONMessages, [ImagePullResponse.Close] does not need to be called in this case.
//   - use the [io.Reader] interface and call [ImagePullResponse.Close] after processing.
func (cli *Client) ImagePull(ctx context.Context, refStr string, options ImagePullOptions) (ImagePullResponse, error) {
	// FIXME(vdemeester): there is currently used in a few way in docker/docker
	// - if not in trusted content, ref is used to pass the whole reference, and tag is empty
	// - if in trusted content, ref is used to pass the reference name, and tag for the digest
	//
	// ref; https://github.com/docker-archive-public/docker.engine-api/pull/162

	ref, err := reference.ParseNormalizedNamed(refStr)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", ref.Name())
	if !options.All {
		query.Set("tag", getAPITagFromNamedRef(ref))
	}
	if len(options.Platforms) > 0 {
		if len(options.Platforms) > 1 {
			// TODO(thaJeztah): update API spec and add equivalent check on the daemon. We need this still for older daemons, which would ignore it.
			return nil, cerrdefs.ErrInvalidArgument.WithMessage("specifying multiple platforms is not yet supported")
		}
		query.Set("platform", formatPlatform(options.Platforms[0]))
	}
	if options.ChallengeHandlerFunc != nil {
		query.Set("clientAuth", "1")
	}

	resp, err := cli.tryImageCreate(ctx, query, staticAuth(options.RegistryAuth))
	if challenge := resp.Header.Get("WWW-Authenticate"); challenge != "" && err != nil {
		if options.ChallengeHandlerFunc != nil {
			var newAuthHeader string
			newAuthHeader, err = options.ChallengeHandlerFunc(ctx, challenge)
			if err != nil {
				return nil, err
			}
			resp, err = cli.tryImageCreate(ctx, query, staticAuth(newAuthHeader))
		}
	}
	if cerrdefs.IsUnauthorized(err) && options.PrivilegeFunc != nil {
		resp, err = cli.tryImageCreate(ctx, query, options.PrivilegeFunc)
	}
	if err != nil {
		return nil, err
	}

	return internal.NewJSONMessageStream(resp.Body), nil
}

// getAPITagFromNamedRef returns a tag from the specified reference.
// This function is necessary as long as the docker "server" api expects
// digests to be sent as tags and makes a distinction between the name
// and tag/digest part of a reference.
func getAPITagFromNamedRef(ref reference.Named) string {
	if digested, ok := ref.(reference.Digested); ok {
		return digested.Digest().String()
	}
	ref = reference.TagNameOnly(ref)
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
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
