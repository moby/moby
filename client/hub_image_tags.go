package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/docker/api/types/hub"
	"github.com/docker/docker/errdefs"
)

func (cli *Client) HubImageTags(ctx context.Context, image string, options hub.ImageOptions) (hub.ImageTags, error) {
	var results hub.ImageTags
	resp, err := cli.tryHubImageGet(ctx, image, "")
	if errdefs.IsUnauthorized(err) && options.PrivilegeFunc != nil {
		newAuthHeader, privilegeErr := options.PrivilegeFunc(ctx)
		if privilegeErr != nil {
			return results, privilegeErr
		}
		resp, err = cli.tryHubImageGet(ctx, image, newAuthHeader)
	}
	defer ensureReaderClosed(resp)

	if err != nil {
		return results, err
	}

	err = json.NewDecoder(resp.body).Decode(&results)
	return results, err
}

func (cli *Client) tryHubImageGet(ctx context.Context, image string, authToken string) (serverResponse, error) {
	return cli.get(ctx, fmt.Sprintf("/hub/image/%s/get", image), nil, http.Header{
		"Authorization": {"Bearer " + authToken},
	})
}
