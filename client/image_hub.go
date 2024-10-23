package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/hub"
	"github.com/docker/docker/errdefs"
)

func (cli *Client) ImageHubTags(ctx context.Context, image string, options hub.ImageOptions) (hub.ImageTags, error) {
	var results hub.ImageTags
	query := options.ToQuery(url.Values{
		"image": {image},
	})
	resp, err := cli.tryImageHubGet(ctx, query, "")
	defer ensureReaderClosed(resp)
	if errdefs.IsUnauthorized(err) && options.PrivilegeFunc != nil {
		newAuthHeader, privilegeErr := options.PrivilegeFunc(ctx)
		if privilegeErr != nil {
			return results, privilegeErr
		}
		resp, err = cli.tryImageSearch(ctx, query, newAuthHeader)
	}

	if err != nil {
		return results, err
	}

	err = json.NewDecoder(resp.body).Decode(&results)
	return results, err
}

func (cli *Client) tryImageHubGet(ctx context.Context, query url.Values, authToken string) (serverResponse, error) {
	return cli.get(ctx, "/images/hub/get", query, http.Header{
		"Authorization": {"Bearer " + authToken},
	})
}
