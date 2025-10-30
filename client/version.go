package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types"
)

// ServerVersionOptions specifies options for the server version request.
type ServerVersionOptions struct {
	// Currently no options are supported.
}

// ServerVersionResult contains information about the Docker server host.
type ServerVersionResult struct {
	// Platform is the platform (product name) the server is running on.
	Platform PlatformInfo

	// APIVersion is the highest API version supported by the server.
	APIVersion string

	// MinAPIVersion is the minimum API version the server supports.
	MinAPIVersion string

	// Components contains version information for the components making
	// up the server. Information in this field is for informational
	// purposes, and not part of the API contract.
	Components []types.ComponentVersion
}

// PlatformInfo holds information about the platform (product name) the
// server is running on.
type PlatformInfo struct {
	// Name is the name of the platform (for example, "Docker Engine - Community",
	// or "Docker Desktop 4.49.0 (208003)")
	Name string
}

// ServerVersion returns information of the Docker server host.
func (cli *Client) ServerVersion(ctx context.Context, _ ServerVersionOptions) (ServerVersionResult, error) {
	resp, err := cli.get(ctx, "/version", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ServerVersionResult{}, err
	}

	var v types.Version
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return ServerVersionResult{}, err
	}

	return ServerVersionResult{
		Platform: PlatformInfo{
			Name: v.Platform.Name,
		},
		APIVersion:    v.APIVersion,
		MinAPIVersion: v.MinAPIVersion,
		Components:    v.Components,
	}, nil
}
