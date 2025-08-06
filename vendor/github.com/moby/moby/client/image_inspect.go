package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

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

	if opts.apiOptions.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.49", "platform"); err != nil {
			return image.InspectResponse{}, err
		}
		platform, err := encodePlatform(opts.apiOptions.Platform)
		if err != nil {
			return image.InspectResponse{}, err
		}
		query.Set("platform", platform)
	}

	resp, err := cli.get(ctx, "/images/"+imageID+"/json", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return image.InspectResponse{}, err
	}

	buf := opts.raw
	if buf == nil {
		buf = &bytes.Buffer{}
	}

	if _, err := io.Copy(buf, resp.Body); err != nil {
		return image.InspectResponse{}, err
	}

	var response image.InspectResponse
	err = json.Unmarshal(buf.Bytes(), &response)
	return response, err
}
