package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// ImageInspect returns the image information.
func (cli *Client) ImageInspect(ctx context.Context, imageID string, inspectOpts ...ImageInspectOption) (ImageInspectResult, error) {
	if imageID == "" {
		return ImageInspectResult{}, objectNotFoundError{object: "image", id: imageID}
	}

	var opts imageInspectOpts
	for _, opt := range inspectOpts {
		if err := opt.Apply(&opts); err != nil {
			return ImageInspectResult{}, fmt.Errorf("error applying image inspect option: %w", err)
		}
	}

	query := url.Values{}
	if opts.apiOptions.Manifests {
		if err := cli.requiresVersion(ctx, "1.48", "manifests"); err != nil {
			return ImageInspectResult{}, err
		}
		query.Set("manifests", "1")
	}

	if opts.apiOptions.Platform != nil {
		if err := cli.requiresVersion(ctx, "1.49", "platform"); err != nil {
			return ImageInspectResult{}, err
		}
		platform, err := encodePlatform(opts.apiOptions.Platform)
		if err != nil {
			return ImageInspectResult{}, err
		}
		query.Set("platform", platform)
	}

	resp, err := cli.get(ctx, "/images/"+imageID+"/json", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ImageInspectResult{}, err
	}

	buf := opts.raw
	if buf == nil {
		buf = &bytes.Buffer{}
	}

	if _, err := io.Copy(buf, resp.Body); err != nil {
		return ImageInspectResult{}, err
	}

	var response ImageInspectResult
	err = json.Unmarshal(buf.Bytes(), &response)
	return response, err
}
