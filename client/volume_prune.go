package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// VolumesPrune requests the daemon to delete unused data
func (cli *Client) VolumesPrune(ctx context.Context, opts volume.PruneOptions) (types.VolumesPruneReport, error) {
	var report types.VolumesPruneReport

	if err := cli.NewVersionError("1.25", "volume prune"); err != nil {
		return report, err
	}

	if versions.GreaterThanOrEqualTo(cli.version, "1.42") {
		if opts.All {
			if opts.Filters.Contains("all") {
				return report, errdefs.InvalidParameter(errors.New(`conflicting options: cannot specify both "all"" and "all" filter"`))
			}
			opts.Filters.Add("all", "true")
		}
	}

	query, err := getFiltersQuery(opts.Filters)
	if err != nil {
		return report, err
	}

	serverResp, err := cli.post(ctx, "/volumes/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return report, err
	}

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving volume prune report: %v", err)
	}

	return report, nil
}
