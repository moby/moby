package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/url"
	"strings"
	"sync"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

func newImagePullResponse(rc io.ReadCloser) ImagePullResponse {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return ImagePullResponse{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

type ImagePullResponse struct {
	rc    io.ReadCloser
	close func() error
}

// Read implements io.ReadCloser
func (r ImagePullResponse) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.ReadCloser
func (r ImagePullResponse) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

// JSONMessages decodes the response stream as a sequence of JSONMessages.
// if stream ends or context is cancelled, the underlying [io.Reader] is closed.
func (r ImagePullResponse) JSONMessages(ctx context.Context) iter.Seq2[jsonmessage.JSONMessage, error] {
	context.AfterFunc(ctx, func() {
		_ = r.Close()
	})
	dec := json.NewDecoder(r)
	return func(yield func(jsonmessage.JSONMessage, error) bool) {
		defer r.Close()
		for {
			var jm jsonmessage.JSONMessage
			err := dec.Decode(&jm)
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				yield(jm, ctx.Err())
				return
			}
			if !yield(jm, err) {
				return
			}
		}
	}
}

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// Callers can use [ImagePullResponse.JSONMessages] to monitor pull progress as
// a sequence of JSONMessages, [ImagePullResponse.Close] does not need to be
// called in this case. Or, use the [io.Reader] interface and call
// [ImagePullResponse.Close] after processing.
func (cli *Client) ImagePull(ctx context.Context, refStr string, options ImagePullOptions) (ImagePullResponse, error) {
	// FIXME(vdemeester): there is currently used in a few way in docker/docker
	// - if not in trusted content, ref is used to pass the whole reference, and tag is empty
	// - if in trusted content, ref is used to pass the reference name, and tag for the digest
	//
	// ref; https://github.com/docker-archive-public/docker.engine-api/pull/162

	ref, err := reference.ParseNormalizedNamed(refStr)
	if err != nil {
		return ImagePullResponse{}, err
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
		return ImagePullResponse{}, err
	}

	return newImagePullResponse(resp.Body), nil
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
