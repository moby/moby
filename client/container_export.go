package client

import (
	"context"
	"io"
	"net/url"
)

// ContainerExportOptions specifies options for container export operations.
type ContainerExportOptions struct {
	// Currently no options are defined for ContainerExport
}

// ContainerExportResult represents the result of a container export operation.
type ContainerExportResult interface {
	io.ReadCloser
}

// ContainerExport retrieves the raw contents of a container
// and returns them as an [io.ReadCloser]. It's up to the caller
// to close the stream.
func (cli *Client) ContainerExport(ctx context.Context, containerID string, options ContainerExportOptions) (ContainerExportResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return nil, err
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/export", url.Values{}, nil)
	if err != nil {
		return nil, err
	}

	return &containerExportResult{
		body: resp.Body,
	}, nil
}

type containerExportResult struct {
	// body must be closed to avoid a resource leak
	body io.ReadCloser
}

var (
	_ io.ReadCloser         = (*containerExportResult)(nil)
	_ ContainerExportResult = (*containerExportResult)(nil)
)

func (r *containerExportResult) Read(p []byte) (int, error) {
	if r == nil || r.body == nil {
		return 0, io.EOF
	}
	return r.body.Read(p)
}

func (r *containerExportResult) Close() error {
	if r == nil || r.body == nil {
		return nil
	}
	return r.body.Close()
}
