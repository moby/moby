package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageCreate creates a new image based on the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(ctx context.Context, parentReference string, options image.CreateOptions) (io.ReadCloser, error) {
	ref, err := reference.ParseNormalizedNamed(parentReference)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", reference.FamiliarName(ref))
	query.Set("tag", getAPITagFromNamedRef(ref))
	if options.Platform != "" {
		query.Set("platform", strings.ToLower(options.Platform))
	}
	resp, err := cli.tryImageCreate(ctx, query, options.RegistryAuth)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) tryImageCreate(ctx context.Context, query url.Values, registryAuth string) (serverResponse, error) {
	return cli.post(ctx, "/images/create", query, nil, http.Header{
		registry.AuthHeader: {registryAuth},
	})
}

func (cli *Client) ImageCreateFromOCIIndex(ctx context.Context, ref reference.NamedTagged, index ocispec.Index) (ocispec.Descriptor, error) {
	query := url.Values{}
	query.Set("fromJSON", "1")
	query.Set("repo", ref.Name())
	query.Set("tag", ref.Tag())

	resp, err := cli.post(ctx, "/images/create", query, index, nil)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	var desc ocispec.Descriptor
	if err := json.NewDecoder(resp.body).Decode(&desc); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}
