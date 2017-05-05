package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/runtime"
	"golang.org/x/net/context"
)

// RuntimeList returns the list of managed runtimes in the Docker host.
func (cli *Client) RuntimeList(ctx context.Context, filter filters.Args) ([]runtime.Info, error) {
	var runtimes runtime.GetRuntimesResponse
	query := url.Values{}

	if filter.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, filter)
		if err != nil {
			return []runtime.Info{}, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/runtimes", query, nil)
	if err != nil {
		return []runtime.Info{}, err
	}

	err = json.NewDecoder(resp.body).Decode(&runtimes)
	ensureReaderClosed(resp)

	return runtimes.Runtimes, err
}
