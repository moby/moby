package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/url"

	"github.com/docker/docker/api/types/network"
)

// NetworkInspectOptions holds parameters to inspect network
type NetworkInspectOptions struct {
	Scope   string
	Verbose bool
}

// NetworkInspect returns the information for a specific network configured in the docker host.
func (cli *Client) NetworkInspect(ctx context.Context, networkID string, options NetworkInspectOptions) (network.Resource, error) {
	networkResource, _, err := cli.NetworkInspectWithRaw(ctx, networkID, options)
	return networkResource, err
}

// NetworkInspectWithRaw returns the information for a specific network configured in the docker host and its raw representation.
func (cli *Client) NetworkInspectWithRaw(ctx context.Context, networkID string, options NetworkInspectOptions) (network.Resource, []byte, error) {
	if networkID == "" {
		return network.Resource{}, nil, objectNotFoundError{object: "network", id: networkID}
	}
	var (
		networkResource network.Resource
		resp            serverResponse
		err             error
	)
	query := url.Values{}
	if options.Verbose {
		query.Set("verbose", "true")
	}
	if options.Scope != "" {
		query.Set("scope", options.Scope)
	}
	resp, err = cli.get(ctx, "/networks/"+networkID, query, nil)
	if err != nil {
		return networkResource, nil, wrapResponseError(err, resp, "network", networkID)
	}
	defer ensureReaderClosed(resp)

	body, err := ioutil.ReadAll(resp.body)
	if err != nil {
		return networkResource, nil, err
	}
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&networkResource)
	return networkResource, body, err
}
