package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/image"
)

// ImageImportOptions holds information to import images from the client host.
type ImageImportOptions struct {
	Tag      string   // Tag is the name to tag this image with. This attribute is deprecated.
	Message  string   // Message is the message to tag the image with
	Changes  []string // Changes are the raw changes to apply to this image
	Platform string   // Platform is the target platform of the image
}

// ImageImport creates a new image based in the source options.
// It returns the JSON content in the response body.
func (cli *Client) ImageImport(ctx context.Context, source image.ImportSource, ref string, options ImageImportOptions) (io.ReadCloser, error) {
	if ref != "" {
		//Check if the given image name can be resolved
		if _, err := reference.ParseNormalizedNamed(ref); err != nil {
			return nil, err
		}
	}

	query := url.Values{}
	query.Set("fromSrc", source.SourceName)
	query.Set("repo", ref)
	query.Set("tag", options.Tag)
	query.Set("message", options.Message)
	if options.Platform != "" {
		query.Set("platform", strings.ToLower(options.Platform))
	}
	for _, change := range options.Changes {
		query.Add("changes", change)
	}

	resp, err := cli.postRaw(ctx, "/images/create", query, source.Source, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
