package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

// ContainerInspectOptions holds options for inspecting a container using
// the [Client.ConfigInspect] method.
type ContainerInspectOptions struct {
	// Size controls whether the container's filesystem size should be calculated.
	// When set, the [container.InspectResponse.SizeRw] and [container.InspectResponse.SizeRootFs]
	// fields in [ContainerInspectResult.Container] are populated with the result.
	//
	// Calculating the size can be a costly operation, and should not be used
	// unless needed.
	Size bool
}

// ContainerInspectResult holds the result from the [Client.ConfigInspect] method.
type ContainerInspectResult struct {
	Container container.InspectResponse
	Raw       json.RawMessage
}

// ContainerInspect returns the container information.
func (cli *Client) ContainerInspect(ctx context.Context, containerID string, options ContainerInspectOptions) (ContainerInspectResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerInspectResult{}, err
	}

	query := url.Values{}
	if options.Size {
		query.Set("size", "1")
	}
	resp, err := cli.get(ctx, "/containers/"+containerID+"/json", query, nil)
	if err != nil {
		return ContainerInspectResult{}, err
	}
	var out ContainerInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Container)
	return out, err
}
