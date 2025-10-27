package client

import (
	"context"
	"net/url"

	"github.com/distribution/reference"
)

// ImageImport creates a new image based on the source options. It returns the
// JSON content in the [ImageImportResult.Body].
//
// If the context is canceled, the underlying [io.ReadCloser] is automatically
// closed.
func (cli *Client) ImageImport(ctx context.Context, source ImageImportSource, ref string, options ImageImportOptions) (ImageImportResult, error) {
	if ref != "" {
		// Check if the given image name can be resolved
		if _, err := reference.ParseNormalizedNamed(ref); err != nil {
			return ImageImportResult{}, err
		}
	}

	query := url.Values{}
	if source.SourceName != "" {
		query.Set("fromSrc", source.SourceName)
	}
	if ref != "" {
		query.Set("repo", ref)
	}
	if options.Tag != "" {
		query.Set("tag", options.Tag)
	}
	if options.Message != "" {
		query.Set("message", options.Message)
	}
	if p := formatPlatform(options.Platform); p != "unknown" {
		// TODO(thaJeztah): would we ever support mutiple platforms here? (would require multiple rootfs tars as well?)
		query.Set("platform", p)
	}
	for _, change := range options.Changes {
		query.Add("changes", change)
	}

	resp, err := cli.postRaw(ctx, "/images/create", query, source.Source, nil)
	if err != nil {
		return ImageImportResult{}, err
	}
	return ImageImportResult{
		Body: newCancelReadCloser(ctx, resp.Body),
	}, nil
}
