package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/registry"
)

// DistributionInspect returns the image digest with the full manifest.
func (cli *Client) DistributionInspect(ctx context.Context, imageRef, encodedRegistryAuth string) (registry.DistributionInspect, error) {
	// Contact the registry to retrieve digest and platform information
	var distributionInspect registry.DistributionInspect
	if imageRef == "" {
		return distributionInspect, objectNotFoundError{object: "distribution", id: imageRef}
	}

	if err := cli.NewVersionError(ctx, "1.30", "distribution inspect"); err != nil {
		return distributionInspect, err
	}

	var headers http.Header
	if encodedRegistryAuth != "" {
		headers = http.Header{
			registry.AuthHeader: {encodedRegistryAuth},
		}
	}

	resp, err := cli.get(ctx, "/distribution/"+imageRef+"/json", url.Values{}, headers)
	defer ensureReaderClosed(resp)
	if err != nil {
		return distributionInspect, err
	}

	err = json.NewDecoder(resp.body).Decode(&distributionInspect)
	return distributionInspect, err
}
