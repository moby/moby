package lib

import (
	"io"
	"net/url"
)

// ContainerStats returns near realtime stats for a given container.
// It's up to the caller to close the io.ReadCloser returned.
func (cli *Client) ContainerStats(containerID string, stream bool) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("stream", "0")
	if stream {
		query.Set("stream", "1")
	}

	resp, err := cli.get("/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, err
}
