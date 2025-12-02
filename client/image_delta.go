package client

import (
	"context"
	"io"
	"net/url"
)

// ImageDeltaResult is the response from ImageDelta operation
type ImageDeltaResult interface {
	io.ReadCloser
}

// ImageDelta generates a binary delta between two images using librsync.
// The delta is stored as a standard OCI image with special metadata labels
// identifying it as a delta and tracking the base image.
//
// baseImage is the source/base image reference (e.g., "busybox:1.24")
// targetImage is the destination/target image reference (e.g., "busybox:1.29")
// options contains the optional tag to apply to the delta image
//
// The delta image will have labels:
//   - io.resin.delta.base: SHA256 of the base image
//   - io.resin.delta.config: Additional delta configuration metadata
func (cli *Client) ImageDelta(ctx context.Context, baseImage, targetImage string, options ImageDeltaOptions) (ImageDeltaResult, error) {
	query := url.Values{
		"src":  []string{baseImage},
		"dest": []string{targetImage},
	}
	if options.Tag != "" {
		query.Set("t", options.Tag)
	}

	resp, err := cli.post(ctx, "/images/delta", query, nil, nil)
	if err != nil {
		return nil, err
	}

	return &imageDeltaResult{
		ReadCloser: newCancelReadCloser(ctx, resp.Body),
	}, nil
}

type imageDeltaResult struct {
	io.ReadCloser
}

var (
	_ io.ReadCloser      = (*imageDeltaResult)(nil)
	_ ImageDeltaResult   = (*imageDeltaResult)(nil)
)
