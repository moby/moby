package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/versions"
)

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (network.CreateResponse, error) {
	// Make sure we negotiated (if the client is configured to do so),
	// as code below contains API-version specific handling of options.
	//
	// Normally, version-negotiation (if enabled) would not happen until
	// the API request is made.
	if err := cli.checkVersion(ctx); err != nil {
		return network.CreateResponse{}, err
	}

	networkCreateRequest := network.CreateRequest{
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

	var req any
	if versions.LessThan(cli.version, "1.44") {
		// CheckDuplicate is removed in API v1.44, and no longer used by
		// daemons supporting that API version (v25.0.0-beta.1 and up)
		// regardless of the API version used, but it must be set to true
		// when sent to older daemons.
		//
		// TODO(thaJeztah) remove this once daemon versions v24.0 and lower are no
		//   longer expected to be used (when Mirantis Container Runtime v23
		//   is EOL);  https://github.com/moby/moby/blob/v2.0.0-beta.0/project/BRANCHES-AND-TAGS.md
		req = struct {
			network.CreateRequest
			CheckDuplicate bool
		}{
			CreateRequest:  networkCreateRequest,
			CheckDuplicate: true,
		}
	} else {
		req = networkCreateRequest
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
