package client

import (
	"encoding/json"
	"net/url"

	registrytypes "github.com/docker/docker/api/types/registry"
	"golang.org/x/net/context"
)

// ImageManifest returns .
func (cli *Client) ImageManifest(ctx context.Context, image, encodedRegistryAuth string) (registrytypes.ManifestInspect, error) {
	var headers map[string][]string

	if encodedRegistryAuth != "" {
		headers = map[string][]string{
			"X-Registry-Auth": {encodedRegistryAuth},
		}
	}

	// Call the /manifest endpoint to retrieve manifest information
	var manifestInspect registrytypes.ManifestInspect
	resp, err := cli.get(ctx, "/images/"+image+"/manifest", url.Values{}, headers)
	if err != nil {
		return manifestInspect, err
	}

	err = json.NewDecoder(resp.body).Decode(&manifestInspect)
	ensureReaderClosed(resp)
	return manifestInspect, err
}
