package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/system"
)

// ServerVersionOptions specifies options for the server version request.
type ServerVersionOptions struct {
	// Currently no options are supported.
}

// ServerVersionResult contains information about the Docker server host.
type ServerVersionResult struct {
	// Platform is the platform (product name) the server is running on.
	Platform PlatformInfo

	// Version is the version of the daemon.
	Version string

	// APIVersion is the highest API version supported by the server.
	APIVersion string

	// MinAPIVersion is the minimum API version the server supports.
	MinAPIVersion string

	// Os is the operating system the server runs on.
	Os string

	// Arch is the hardware architecture the server runs on.
	Arch string

	// Experimental indicates that the daemon runs with experimental
	// features enabled.
	//
	// Deprecated: this field will be removed in the next version.
	Experimental bool

	// Components contains version information for the components making
	// up the server. Information in this field is for informational
	// purposes, and not part of the API contract.
	Components []system.ComponentVersion
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

	var v system.VersionResponse
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return ServerVersionResult{}, err
	}

	return ServerVersionResult{
		Platform: PlatformInfo{
			Name: v.Platform.Name,
		},
		Version:       v.Version,
		APIVersion:    v.APIVersion,
		MinAPIVersion: v.MinAPIVersion,
		Os:            v.Os,
		Arch:          v.Arch,
		Experimental:  v.Experimental, //nolint:staticcheck // ignore deprecated field.
		Components:    v.Components,
	}, nil
}
