package client

import "golang.org/x/net/context"

// Ping pings the server and return the value of the "Docker-Experimental" header
func (cli *Client) Ping(ctx context.Context) (bool, error) {
	serverResp, err := cli.get(ctx, "/_ping", nil, nil)
	if err != nil {
		return false, err
	}
	defer ensureReaderClosed(serverResp)

	exp := serverResp.header.Get("Docker-Experimental")
	if exp != "true" {
		return false, nil
	}

	return true, nil
}
