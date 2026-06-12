package client

import (
	"context"
	"encoding/json"
	"net/url"
)

// ImageAttestations returns the in-toto attestation statements attached to an
// image for the given platform. This requires API version 1.55 or higher.
func (cli *Client) ImageAttestations(ctx context.Context, imageID string, opts ...ImageAttestationsOption) (ImageAttestationsResult, error) {
	if imageID == "" {
		return ImageAttestationsResult{}, objectNotFoundError{object: "image", id: imageID}
	}

	if err := cli.requiresVersion(ctx, "1.55", "attestations"); err != nil {
		return ImageAttestationsResult{}, err
	}

	var o imageAttestationsOpts
	for _, opt := range opts {
		if err := opt.Apply(&o); err != nil {
			return ImageAttestationsResult{}, err
		}
	}

	query := url.Values{}
	if o.platform != nil {
		p, err := encodePlatform(o.platform)
		if err != nil {
			return ImageAttestationsResult{}, err
		}
		query.Set("platform", p)
	}
	for _, pt := range o.predicateTypes {
		query.Add("type", pt)
	}
	if o.includeStatement {
		query.Set("statement", "1")
	}

	resp, err := cli.get(ctx, "/images/"+imageID+"/attestations", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ImageAttestationsResult{}, err
	}

	var result ImageAttestationsResult
	err = json.NewDecoder(resp.Body).Decode(&result.Items)
	return result, err
}
