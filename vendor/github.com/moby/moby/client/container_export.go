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
type ContainerExportResult struct {
	Body io.ReadCloser
}

// ContainerExport retrieves the raw contents of a container and returns them
// as an [io.ReadCloser] in ContainerExportResult.Body. Callers should close
// the stream, but the underlying [io.ReadCloser] is automatically closed
// if the context is canceled,
func (cli *Client) ContainerExport(ctx context.Context, containerID string, options ContainerExportOptions) (ContainerExportResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerExportResult{}, err
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/export", url.Values{}, nil)
	if err != nil {
		return ContainerExportResult{}, err
	}

	return ContainerExportResult{Body: newCancelReadCloser(ctx, resp.Body)}, nil
}
