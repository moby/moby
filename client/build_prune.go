package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/filters"
)

// BuildCachePrune requests the daemon to delete unused cache data.
func (cli *Client) BuildCachePrune(ctx context.Context, opts build.CachePruneOptions) (*build.CachePruneReport, error) {
	if err := cli.NewVersionError(ctx, "1.31", "build prune"); err != nil {
		return nil, err
	}

	query := url.Values{}
	if opts.All {
		query.Set("all", "1")
	}

	if opts.ReservedSpace != 0 {
		query.Set("reserved-space", strconv.Itoa(int(opts.ReservedSpace)))
	}
	if opts.MaxUsedSpace != 0 {
		query.Set("max-used-space", strconv.Itoa(int(opts.MaxUsedSpace)))
	}
	if opts.MinFreeSpace != 0 {
		query.Set("min-free-space", strconv.Itoa(int(opts.MinFreeSpace)))
	}
	f, err := filters.ToJSON(opts.Filters)
	if err != nil {
		return nil, fmt.Errorf("prune could not marshal filters option: %w", err)
	}
	query.Set("filters", f)

	resp, err := cli.post(ctx, "/build/prune", query, nil, nil)
	defer ensureReaderClosed(resp)

	if err != nil {
		return nil, err
	}

	report := build.CachePruneReport{}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("error retrieving disk usage: %w", err)
	}

	return &report, nil
}
