package client

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
)

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// It's up to the caller to handle the io.ReadCloser and close it properly.
//
// FIXME(vdemeester): there is currently used in a few way in docker/docker
// - if not in trusted content, ref is used to pass the whole reference, and tag is empty
// - if in trusted content, ref is used to pass the reference name, and tag for the digest
func (cli *Client) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
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
	resp, err := cli.tryImageCreate(ctx, query, ChainPrivilegeFuncs(staticAuth(options.RegistryAuth), options.PrivilegeFunc))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
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
