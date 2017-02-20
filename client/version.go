package client

import (
	"encoding/json"

	"github.com/docker/docker/api/types/system"
	"golang.org/x/net/context"
)

// ServerVersion returns information of the docker client and server host.
func (cli *Client) ServerVersion(ctx context.Context) (system.VersionOKBody, error) {
	resp, err := cli.get(ctx, "/version", nil, nil)
	if err != nil {
		return system.VersionOKBody{}, err
	}

	var server system.VersionOKBody
	err = json.NewDecoder(resp.body).Decode(&server)
	ensureReaderClosed(resp)
	return server, err
}
