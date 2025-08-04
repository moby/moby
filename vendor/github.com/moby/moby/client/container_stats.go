package client

import (
	"context"
	"io"
	"net/url"
)

// StatsResponseReader wraps an [io.ReadCloser] to read (a stream of) stats
// for a container, as produced by the GET "/stats" endpoint.
//
// The OSType field is set to the server's platform to allow
// platform-specific handling of the response.
//
// TODO(thaJeztah): remove this wrapper, and make OSType part of [github.com/moby/moby/api/types/container.StatsResponse].
type StatsResponseReader struct {
	Body   io.ReadCloser `json:"body"`
	OSType string        `json:"ostype"`
}

// ContainerStats returns near realtime stats for a given container.
// It's up to the caller to close the [io.ReadCloser] returned.
func (cli *Client) ContainerStats(ctx context.Context, containerID string, stream bool) (StatsResponseReader, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return StatsResponseReader{}, err
	}

	query := url.Values{}
	query.Set("stream", "0")
	if stream {
		query.Set("stream", "1")
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return StatsResponseReader{}, err
	}

	return StatsResponseReader{
		Body:   resp.Body,
		OSType: resp.Header.Get("Ostype"),
	}, nil
}

// ContainerStatsOneShot gets a single stat entry from a container.
// It differs from `ContainerStats` in that the API should not wait to prime the stats
func (cli *Client) ContainerStatsOneShot(ctx context.Context, containerID string) (StatsResponseReader, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return StatsResponseReader{}, err
	}

	query := url.Values{}
	query.Set("stream", "0")
	query.Set("one-shot", "1")

	resp, err := cli.get(ctx, "/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return StatsResponseReader{}, err
	}

	return StatsResponseReader{
		Body:   resp.Body,
		OSType: resp.Header.Get("Ostype"),
	}, nil
}
