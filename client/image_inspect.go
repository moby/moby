package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/docker/docker/api/types/image"
)

// ImageInspectOption is a type representing functional options for the image inspect operation.
type ImageInspectOption interface {
	Apply(*imageInspectOpts) error
}
type imageInspectOptionFunc func(opt *imageInspectOpts) error

func (f imageInspectOptionFunc) Apply(o *imageInspectOpts) error {
	return f(o)
}

// ImageInspectWithRawResponse instructs the client to additionally store the
// raw inspect response in the provided buffer.
func ImageInspectWithRawResponse(raw *bytes.Buffer) ImageInspectOption {
	return imageInspectOptionFunc(func(opts *imageInspectOpts) error {
		opts.raw = raw
		return nil
	})
}

// ImageInspectWithManifests sets manifests API option for the image inspect operation.
// This option is only available for API version 1.48 and up.
// With this option set, the image inspect operation response will have the
// [image.InspectResponse.Manifests] field populated if the server is multi-platform capable.
func ImageInspectWithManifests(manifests bool) ImageInspectOption {
	return imageInspectOptionFunc(func(clientOpts *imageInspectOpts) error {
		clientOpts.apiOptions.Manifests = manifests
		return nil
	})
}

// ImageInspectWithAPIOpts sets the API options for the image inspect operation.
func ImageInspectWithAPIOpts(opts image.InspectOptions) ImageInspectOption {
	return imageInspectOptionFunc(func(clientOpts *imageInspectOpts) error {
		clientOpts.apiOptions = opts
		return nil
	})
}

type imageInspectOpts struct {
	raw        *bytes.Buffer
	apiOptions image.InspectOptions
}

// ImageInspect returns the image information.
func (cli *Client) ImageInspect(ctx context.Context, imageID string, inspectOpts ...ImageInspectOption) (image.InspectResponse, error) {
	if imageID == "" {
		return image.InspectResponse{}, objectNotFoundError{object: "image", id: imageID}
	}

	var opts imageInspectOpts
	for _, opt := range inspectOpts {
		if err := opt.Apply(&opts); err != nil {
			return image.InspectResponse{}, fmt.Errorf("error applying image inspect option: %w", err)
		}
	}

	query := url.Values{}
	if opts.apiOptions.Manifests {
		if err := cli.NewVersionError(ctx, "1.48", "manifests"); err != nil {
			return image.InspectResponse{}, err
		}
		query.Set("manifests", "1")
	}

	serverResp, err := cli.get(ctx, "/images/"+imageID+"/json", query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return image.InspectResponse{}, err
	}

	buf := opts.raw
	if buf == nil {
		buf = &bytes.Buffer{}
	}

	if _, err := io.Copy(buf, serverResp.body); err != nil {
		return image.InspectResponse{}, err
	}

	var response image.InspectResponse
	err = json.Unmarshal(buf.Bytes(), &response)
	return response, err
}

// ImageInspectWithRaw returns the image information and its raw representation.
//
// Deprecated: Use [Client.ImageInspect] instead.
// Raw response can be obtained by [ImageInspectWithRawResponse] option.
func (cli *Client) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	var buf bytes.Buffer
	resp, err := cli.ImageInspect(ctx, imageID, ImageInspectWithRawResponse(&buf))
	if err != nil {
		return image.InspectResponse{}, nil, err
	}
	return resp, buf.Bytes(), err
}
