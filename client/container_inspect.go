package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/url"

	"github.com/docker/docker/api/types/container"
)

// ContainerInspect returns the container information.
func (cli *Client) ContainerInspect(ctx context.Context, containerID string) (container.JSON, error) {
	if containerID == "" {
		return container.JSON{}, objectNotFoundError{object: "container", id: containerID}
	}
	serverResp, err := cli.get(ctx, "/containers/"+containerID+"/json", nil, nil)
	if err != nil {
		return container.JSON{}, wrapResponseError(err, serverResp, "container", containerID)
	}

	var response container.JSON
	err = json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}

// ContainerInspectWithRaw returns the container information and its raw representation.
func (cli *Client) ContainerInspectWithRaw(ctx context.Context, containerID string, getSize bool) (container.JSON, []byte, error) {
	if containerID == "" {
		return container.JSON{}, nil, objectNotFoundError{object: "container", id: containerID}
	}
	query := url.Values{}
	if getSize {
		query.Set("size", "1")
	}
	serverResp, err := cli.get(ctx, "/containers/"+containerID+"/json", query, nil)
	if err != nil {
		return container.JSON{}, nil, wrapResponseError(err, serverResp, "container", containerID)
	}
	defer ensureReaderClosed(serverResp)

	body, err := ioutil.ReadAll(serverResp.body)
	if err != nil {
		return container.JSON{}, nil, err
	}

	var response container.JSON
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&response)
	return response, body, err
}
