package client

import (
	"context"
	"io"
	"net/url"
	"sync"
)

// ContainerExportOptions specifies options for container export operations.
type ContainerExportOptions struct {
	// Currently no options are defined for ContainerExport
}

// ContainerExportResult represents the result of a container export operation.
type ContainerExportResult struct {
	rc    io.ReadCloser
	close func() error
}

// ContainerExport retrieves the raw contents of a container
// and returns them as an [io.ReadCloser]. It's up to the caller
// to close the stream.
func (cli *Client) ContainerExport(ctx context.Context, containerID string, options ContainerExportOptions) (ContainerExportResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerExportResult{}, err
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/export", url.Values{}, nil)
	if err != nil {
		return ContainerExportResult{}, err
	}

	return newContainerExportResult(resp.Body), nil
}

func newContainerExportResult(rc io.ReadCloser) ContainerExportResult {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return ContainerExportResult{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

// Read implements io.ReadCloser
func (r ContainerExportResult) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.ReadCloser
func (r ContainerExportResult) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}
