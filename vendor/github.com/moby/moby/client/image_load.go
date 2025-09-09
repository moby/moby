package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// ImageLoad loads an image in the docker host from the client host.
// It's up to the caller to close the [io.ReadCloser] in the
// [image.LoadResponse] returned by this function.
//
// Platform is an optional parameter that specifies the platform to load from
// the provided multi-platform image. Passing a platform only has an effect
// if the input image is a multi-platform image.
func (cli *Client) ImageLoad(ctx context.Context, input io.Reader, loadOpts ...ImageLoadOption) (LoadResponse, error) {
	var opts imageLoadOpts
	for _, opt := range loadOpts {
		if err := opt.Apply(&opts); err != nil {
			return LoadResponse{}, err
		}
	}

	query := url.Values{}
	query.Set("quiet", "0")
	if opts.apiOptions.Quiet {
		query.Set("quiet", "1")
	}
	if len(opts.apiOptions.Platforms) > 0 {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return LoadResponse{}, err
		}

		p, err := encodePlatforms(opts.apiOptions.Platforms...)
		if err != nil {
			return LoadResponse{}, err
		}
		query["platform"] = p
	}

	resp, err := cli.postRaw(ctx, "/images/load", query, input, http.Header{
		"Content-Type": {"application/x-tar"},
	})
	if err != nil {
		return LoadResponse{}, err
	}
	return LoadResponse{
		Body: resp.Body,
		JSON: resp.Header.Get("Content-Type") == "application/json",
	}, nil
}

// LoadResponse returns information to the client about a load process.
//
// TODO(thaJeztah): remove this type, and just use an io.ReadCloser
//
// This type was added in https://github.com/moby/moby/pull/18878, related
// to https://github.com/moby/moby/issues/19177;
//
// Make docker load to output json when the response content type is json
// Swarm hijacks the response from docker load and returns JSON rather
// than plain text like the Engine does. This makes the API library to return
// information to figure that out.
//
// However the "load" endpoint unconditionally returns JSON;
// https://github.com/moby/moby/blob/7b9d2ef6e5518a3d3f3cc418459f8df786cfbbd1/api/server/router/image/image_routes.go#L248-L255
//
// PR https://github.com/moby/moby/pull/21959 made the response-type depend
// on whether "quiet" was set, but this logic got changed in a follow-up
// https://github.com/moby/moby/pull/25557, which made the JSON response-type
// unconditionally, but the output produced depend on whether"quiet" was set.
//
// We should deprecated the "quiet" option, as it's really a client
// responsibility.
type LoadResponse struct {
	// Body must be closed to avoid a resource leak
	Body io.ReadCloser
	JSON bool
}
