package client

import (
	"context"
	"io"
	"net/url"
)

// ContainerStatsOptions holds parameters to retrieve container statistics
// using the [Client.ContainerStats] method.
type ContainerStatsOptions struct {
	// Stream enables streaming [container.StatsResponse] results instead
	// of collecting a single sample. If enabled, the client remains attached
	// until the [ContainerStatsResult.Body] is closed or the context is
	// cancelled.
	Stream bool

	// IncludePreviousSample asks the daemon to  collect a prior sample to populate the
	// [container.StatsResponse.PreRead] and [container.StatsResponse.PreCPUStats]
	// fields.
	//
	// It set, the daemon collects two samples at a one-second interval before
	// returning the result. The first sample populates the PreCPUStats (“previous
	// CPU”) field, allowing delta calculations for CPU usage. If false, only
	// a single sample is taken and returned immediately, leaving PreRead and
	// PreCPUStats empty.
	//
	// This option has no effect if Stream is enabled. If Stream is enabled,
	// [container.StatsResponse.PreCPUStats] is never populated for the first
	// record.
	IncludePreviousSample bool
}

// ContainerStatsResult holds the result from [Client.ContainerStats].
//
// It wraps an [io.ReadCloser] that provides one or more [container.StatsResponse]
// objects for a container, as produced by the "GET /containers/{id}/stats" endpoint.
// If streaming is disabled, the stream contains a single record.
type ContainerStatsResult struct {
	Body io.ReadCloser
}

// ContainerStats retrieves live resource usage statistics for the specified
// container. The caller must close the [io.ReadCloser] in the returned result
// to release associated resources.
func (cli *Client) ContainerStats(ctx context.Context, containerID string, options ContainerStatsOptions) (ContainerStatsResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerStatsResult{}, err
	}

	query := url.Values{}
	if options.Stream {
		query.Set("stream", "true")
	} else {
		// Note: daemons before v29.0 return an error if both set: "cannot have stream=true and one-shot=true"
		//
		// TODO(thaJeztah): consider making "stream=false" the default for the API as well, or using Accept Header to switch.
		query.Set("stream", "false")
		if !options.IncludePreviousSample {
			query.Set("one-shot", "true")
		}
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return ContainerStatsResult{}, err
	}

	return ContainerStatsResult{
		Body: resp.Body,
	}, nil
}
