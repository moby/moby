package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

// SwarmInit initializes the Swarm.
func (cli *Client) SwarmInit(ctx context.Context, req swarm.InitRequest) (string, error) {
	serverResp, err := cli.post(ctx, "/swarm/init", nil, req, nil)
	if err != nil {
		return "", err
	}

	var response string
	err = json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}

// SwarmJoin joins the Swarm.
func (cli *Client) SwarmJoin(ctx context.Context, req swarm.JoinRequest) error {
	resp, err := cli.post(ctx, "/swarm/join", nil, req, nil)
	ensureReaderClosed(resp)
	return err
}

// SwarmLeave leaves the Swarm.
func (cli *Client) SwarmLeave(ctx context.Context, force bool) error {
	query := url.Values{}
	if force {
		query.Set("force", "1")
	}
	resp, err := cli.post(ctx, "/swarm/leave", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}

// SwarmInspect inspects the Swarm.
func (cli *Client) SwarmInspect(ctx context.Context) (swarm.Swarm, error) {
	serverResp, err := cli.get(ctx, "/swarm", nil, nil)
	if err != nil {
		return swarm.Swarm{}, err
	}

	var response swarm.Swarm
	err = json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}

// SwarmUpdate updates the Swarm.
func (cli *Client) SwarmUpdate(ctx context.Context, swarm swarm.Swarm) error {
	resp, err := cli.post(ctx, "/swarm/update", nil, swarm, nil)
	ensureReaderClosed(resp)
	return err
}
