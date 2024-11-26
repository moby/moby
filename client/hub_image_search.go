package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/hub"
	"github.com/docker/docker/errdefs"
)

func (cli *Client) HubImageSearch(ctx context.Context, term string, options hub.SearchOptions) (hub.SearchResult, error) {
	var results hub.SearchResult
	query := options.ToQuery(url.Values{
		"query": {term},
	})
	resp, err := cli.tryHubImageSearch(ctx, query, "")
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

func (cli *Client) tryHubImageSearch(ctx context.Context, query url.Values, authToken string) (serverResponse, error) {
	return cli.get(ctx, "/hub/image/search", query, http.Header{
		"Authorization": {"Bearer " + authToken},
	})
}
