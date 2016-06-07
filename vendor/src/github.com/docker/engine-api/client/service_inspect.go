package client

import (
	"encoding/json"
	"net/http"

	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

// ServiceInspect returns the service information.
func (cli *Client) ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error) {
	serverResp, err := cli.get(ctx, "/services/"+serviceID, nil, nil)
	if err != nil {
		if serverResp.statusCode == http.StatusNotFound {
			return swarm.Service{}, serviceNotFoundError{serviceID}
		}
		return swarm.Service{}, err
	}

	var response swarm.Service
	err = json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}
