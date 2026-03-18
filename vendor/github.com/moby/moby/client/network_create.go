package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

// NetworkCreateOptions holds options to create a network.
type NetworkCreateOptions struct {
	Driver     string            // Driver is the driver-name used to create the network (e.g. `bridge`, `overlay`)
	Scope      string            // Scope describes the level at which the network exists (e.g. `swarm` for cluster-wide or `local` for machine level).
	EnableIPv4 *bool             // EnableIPv4 represents whether to enable IPv4.
	EnableIPv6 *bool             // EnableIPv6 represents whether to enable IPv6.
	IPAM       *network.IPAM     // IPAM is the network's IP Address Management.
	Internal   bool              // Internal represents if the network is used internal only.
	Attachable bool              // Attachable represents if the global scope is manually attachable by regular containers from workers in swarm mode.
	Ingress    bool              // Ingress indicates the network is providing the routing-mesh for the swarm cluster.
	ConfigOnly bool              // ConfigOnly creates a config-only network. Config-only networks are place-holder networks for network configurations to be used by other networks. ConfigOnly networks cannot be used directly to run containers or services.
	ConfigFrom string            // ConfigFrom specifies the source which will provide the configuration for this network. The specified network must be a config-only network; see [CreateOptions.ConfigOnly].
	Options    map[string]string // Options specifies the network-specific options to use for when creating the network.
	Labels     map[string]string // Labels holds metadata specific to the network being created.
}

// NetworkCreateResult represents the result of a network create operation.
type NetworkCreateResult struct {
	ID string

	Warning []string
}

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (NetworkCreateResult, error) {
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
		Options:    options.Options,
		Labels:     options.Labels,
	}

	if options.ConfigFrom != "" {
		req.ConfigFrom = &network.ConfigReference{Network: options.ConfigFrom}
	}

	resp, err := cli.post(ctx, "/networks/create", nil, req, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return NetworkCreateResult{}, err
	}

	var response network.CreateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)

	var warnings []string
	if response.Warning != "" {
		warnings = []string{response.Warning}
	}

	return NetworkCreateResult{ID: response.ID, Warning: warnings}, err
}
