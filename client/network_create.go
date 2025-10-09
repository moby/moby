package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (network.CreateResponse, error) {
	req := network.CreateRequest{
		Name:       name,
		Driver:     options.Driver,
		Scope:      options.Scope,
		EnableIPv4: options.EnableIPv4,
		EnableIPv6: options.EnableIPv6,
		IPAM:       options.IPAM,
		Internal:   options.Internal,
		Attachable: options.Attachable,
		Ingress:    options.Ingress,
		ConfigOnly: options.ConfigOnly,
		ConfigFrom: options.ConfigFrom,
		Options:    options.Options,
		Labels:     options.Labels,
	}

	resp, err := cli.post(ctx, "/networks/create", nil, req, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return network.CreateResponse{}, err
	}

	var response network.CreateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
}
