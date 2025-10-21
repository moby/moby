package client

import (
	"context"
	"io"
	"iter"
	"net/url"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/moby/moby/client/internal"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

type ImagePullResponse interface {
	io.ReadCloser
	JSONMessages(ctx context.Context) iter.Seq2[jsonmessage.JSONMessage, error]
	Wait(ctx context.Context) error
}

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// Callers can:
// - use [ImagePullResponse.Wait] to wait for pull to complete
// - use [ImagePullResponse.JSONMessages] to monitor pull progress as a sequence
//   of JSONMessages, [ImagePullResponse.Close] does not need to be called in this case.
// - use the [io.Reader] interface and call [ImagePullResponse.Close] after processing.
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
	if options.Platform != "" {
		query.Set("platform", strings.ToLower(options.Platform))
	}

	resp, err := cli.tryImageCreate(ctx, query, staticAuth(options.RegistryAuth))
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
