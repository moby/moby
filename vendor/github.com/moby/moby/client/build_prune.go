package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/versions"
)

// BuildCachePruneOptions hold parameters to prune the build cache.
type BuildCachePruneOptions struct {
	All           bool
	ReservedSpace int64
	MaxUsedSpace  int64
	MinFreeSpace  int64
	Filters       filters.Args
}

// BuildCachePrune requests the daemon to delete unused cache data.
func (cli *Client) BuildCachePrune(ctx context.Context, opts BuildCachePruneOptions) (*build.CachePruneReport, error) {
	query := url.Values{}
	if opts.All {
		query.Set("all", "1")
	}

	if opts.ReservedSpace != 0 {
		// Prior to API v1.48, 'keep-storage' was used to set the reserved space for the build cache.
		// TODO(austinvazquez): remove once API v1.47 is no longer supported. See https://github.com/moby/moby/issues/50902
		if versions.LessThanOrEqualTo(cli.version, "1.47") {
			query.Set("keep-storage", strconv.Itoa(int(opts.ReservedSpace)))
		} else {
			query.Set("reserved-space", strconv.Itoa(int(opts.ReservedSpace)))
		}
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
