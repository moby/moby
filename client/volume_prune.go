package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// VolumesPrune requests the daemon to delete unused data
func (cli *Client) VolumesPrune(ctx context.Context, opts volume.PruneOptions) (volume.PruneReport, error) {
	if err := cli.NewVersionError(ctx, "1.25", "volume prune"); err != nil {
		return volume.PruneReport{}, err
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
		return volume.PruneReport{}, err
	}

	serverResp, err := cli.post(ctx, "/volumes/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return volume.PruneReport{}, err
	}

	var report volume.PruneReport
	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return volume.PruneReport{}, fmt.Errorf("Error retrieving volume prune report: %v", err)
	}

	return report, nil
}
