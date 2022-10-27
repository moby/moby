package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/pkg/errors"
)

// BuildCachePrune requests the daemon to delete unused cache data
func (cli *Client) BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error) {
	if err := cli.NewVersionError("1.31", "build prune"); err != nil {
		return nil, err
	}

	report := types.BuildCachePruneReport{}

	query := url.Values{}
	if opts.All {
		query.Set("all", "1")
	}
	query.Set("keep-storage", strconv.Itoa(int(opts.KeepStorage)))
	f, err := filters.ToJSON(opts.Filters)
	if err != nil {
		return nil, errors.Wrap(err, "prune could not marshal filters option")
	}
	query.Set("filters", f)

	serverResp, err := cli.post(ctx, "/build/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)

	if err != nil {
		return nil, err
	}

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return nil, errors.Wrap(err, "error retrieving disk usage")
	}

	return &report, nil
}
