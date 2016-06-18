package client

import (
	"encoding/json"
	"net/http"

	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

// NodeInspect returns the node information.
func (cli *Client) NodeInspect(ctx context.Context, nodeID string) (swarm.Node, error) {
	serverResp, err := cli.get(ctx, "/nodes/"+nodeID, nil, nil)
	if err != nil {
		if serverResp.statusCode == http.StatusNotFound {
			return swarm.Node{}, nodeNotFoundError{nodeID}
		}
		return swarm.Node{}, err
	}

	var response swarm.Node
	err = json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}
