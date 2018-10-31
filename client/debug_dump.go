package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/url"
)

// GenerateSupportDump requests the daemon to send a support dump.
// The returned stream is a tar+gz containing information that is often requested
// when debugging issues.
// This should be suitable for submitting with bug reports.
func (cli *Client) GenerateSupportDump(ctx context.Context) (io.ReadCloser, error) {
	resp, err := cli.post(ctx, "/debug/dump", url.Values{}, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
