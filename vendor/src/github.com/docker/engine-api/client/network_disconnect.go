package client

import (
	"github.com/docker/engine-api/types"
)

// NetworkDisconnect disconnects a container from an existent network in the docker host.
func (cli *Client) NetworkDisconnect(networkID, containerID string, force bool) error {
	nd := types.NetworkDisconnect{Container: containerID, Force: force}
	resp, err := cli.post("/networks/"+networkID+"/disconnect", nil, nd, nil)
	ensureReaderClosed(resp)
	return err
}
