package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"

	"github.com/docker/docker/api/types/container"
)

// ContainerInspect returns the container information.
func (cli *Client) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if containerID == "" {
		return container.InspectResponse{}, objectNotFoundError{object: "container", id: containerID}
	}
	serverResp, err := cli.get(ctx, "/containers/"+containerID+"/json", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return container.InspectResponse{}, err
	}

	var response container.InspectResponse
	err = json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}

// ContainerInspectWithRaw returns the container information and its raw representation.
func (cli *Client) ContainerInspectWithRaw(ctx context.Context, containerID string, getSize bool) (container.InspectResponse, []byte, error) {
	if containerID == "" {
		return container.InspectResponse{}, nil, objectNotFoundError{object: "container", id: containerID}
	}
	query := url.Values{}
	if getSize {
		query.Set("size", "1")
	}
	serverResp, err := cli.get(ctx, "/containers/"+containerID+"/json", query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return container.InspectResponse{}, nil, err
	}

	body, err := io.ReadAll(serverResp.body)
	if err != nil {
		return container.InspectResponse{}, nil, err
	}

	var response container.InspectResponse
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&response)
	return response, body, err
}
