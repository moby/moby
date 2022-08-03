package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// PluginList returns the installed plugins
func (cli *Client) PluginList(ctx context.Context, filter filters.Args) (types.PluginsListResponse, error) {
	var plugins types.PluginsListResponse
	query := url.Values{}

	req, err := cli.versioned(ctx)
	if err != nil {
		return plugins, err
	}
	if filter.Len() > 0 {
		//nolint:staticcheck // ignore SA1019 for old code
		filterJSON, err := filters.ToParamWithVersion(req.version, filter)
		if err != nil {
			return plugins, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := req.get(ctx, "/plugins", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return plugins, err
	}

	err = json.NewDecoder(resp.body).Decode(&plugins)
	return plugins, err
}
