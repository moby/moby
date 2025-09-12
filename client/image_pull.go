package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// If a `ProgressFunc` is set, ImagePull decodes pull messages from API stream and ImagePull only
// return after pull operation completed. Otherwise raw API stream is returned as an [io.ReadCloser]
// It's then up to the caller to handle and close it.
func (cli *Client) ImagePull(ctx context.Context, refStr string, options ImagePullOptions) (io.ReadCloser, error) {
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

	stream := resp.Body
	if options.ProgressFunc == nil {
		return stream, nil
	}

	defer stream.Close()
	dec := json.NewDecoder(stream)
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		options.ProgressFunc(jm)
		if jm.Error != nil {
			err = jm.Error
		}
	}
	return nil, err
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
