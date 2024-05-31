package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
)

// NetworkInspect returns the information for a specific network configured in the docker host.
func (cli *Client) NetworkInspect(ctx context.Context, networkID string, options network.InspectOptions) (types.NetworkResource, error) {
	networkResource, _, err := cli.NetworkInspectWithRaw(ctx, networkID, options)
	return networkResource, err
}

// NetworkInspectWithRaw returns the information for a specific network configured in the docker host and its raw representation.
func (cli *Client) NetworkInspectWithRaw(ctx context.Context, networkID string, options network.InspectOptions) (types.NetworkResource, []byte, error) {
	if networkID == "" {
		return types.NetworkResource{}, nil, objectNotFoundError{object: "network", id: networkID}
	}
	query := url.Values{}
	if options.Verbose {
		query.Set("verbose", "true")
	}
	if options.Scope != "" {
		query.Set("scope", options.Scope)
	}

	resp, err := cli.get(ctx, "/networks/"+networkID, query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return types.NetworkResource{}, nil, err
	}

	raw, err := io.ReadAll(resp.body)
	if err != nil {
		return types.NetworkResource{}, nil, err
	}

	var nw types.NetworkResource
	err = json.NewDecoder(bytes.NewReader(raw)).Decode(&nw)
	return nw, raw, err
}
