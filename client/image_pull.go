package client

import (
	"io"
	"net/http"
	"net/url"

	"golang.org/x/net/context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/system"
)

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// It's up to the caller to handle the io.ReadCloser and close it properly.
//
// FIXME(vdemeester): there is currently used in a few way in docker/docker
// - if not in trusted content, ref is used to pass the whole reference, and tag is empty
// - if in trusted content, ref is used to pass the reference name, and tag for the digest
func (cli *Client) ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(refStr)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", reference.FamiliarName(ref))
	if !options.All {
		query.Set("tag", getAPITagFromNamedRef(ref))
	}

	// TODO 1: Extend to include "and the platform is supported by the daemon".
	// This is dependent on https://github.com/moby/moby/pull/34628 though,
	// and the daemon returning the set of platforms it supports via the _ping
	// API endpoint.
	//
	// TODO 2: system.IsPlatformEmpty is a temporary function. We need to move
	// (in the reasonably short future) to a package which supports all the platform
	// validation such as is proposed in https://github.com/containerd/containerd/pull/1403
	//
	// @jhowardmsft.
	if !system.IsPlatformEmpty(options.Platform) {
		if err := cli.NewVersionError("1.32", "platform"); err != nil {
			return nil, err
		}
	}

	resp, err := cli.tryImageCreate(ctx, query, options.RegistryAuth, options.Platform)
	if resp.statusCode == http.StatusUnauthorized && options.PrivilegeFunc != nil {
		newAuthHeader, privilegeErr := options.PrivilegeFunc()
		if privilegeErr != nil {
			return nil, privilegeErr
		}
		resp, err = cli.tryImageCreate(ctx, query, newAuthHeader, options.Platform)
	}
	if err != nil {
		return nil, err
	}
	return resp.body, nil
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
