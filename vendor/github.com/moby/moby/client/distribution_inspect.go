package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/moby/moby/api/types/registry"
)

// DistributionInspectResult holds the result of the DistributionInspect operation.
type DistributionInspectResult struct {
	registry.DistributionInspect
}

// DistributionInspectOptions holds options for the DistributionInspect operation.
type DistributionInspectOptions struct {
	EncodedRegistryAuth string
}

// DistributionInspect returns the image digest with the full manifest.
func (cli *Client) DistributionInspect(ctx context.Context, imageRef string, options DistributionInspectOptions) (DistributionInspectResult, error) {
	if imageRef == "" {
		return DistributionInspectResult{}, objectNotFoundError{object: "distribution", id: imageRef}
	}

	var headers http.Header
	if options.EncodedRegistryAuth != "" {
		headers = http.Header{
			registry.AuthHeader: {options.EncodedRegistryAuth},
		}
	}

	// Contact the registry to retrieve digest and platform information
	resp, err := cli.get(ctx, "/distribution/"+imageRef+"/json", url.Values{}, headers)
	defer ensureReaderClosed(resp)
	if err != nil {
		return DistributionInspectResult{}, err
	}

	var distributionInspect registry.DistributionInspect
	err = json.NewDecoder(resp.Body).Decode(&distributionInspect)
	return DistributionInspectResult{DistributionInspect: distributionInspect}, err
}
