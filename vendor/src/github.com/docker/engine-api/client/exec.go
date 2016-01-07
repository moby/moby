package client

import (
	"encoding/json"

	"github.com/docker/engine-api/types"
)

// ContainerExecCreate creates a new exec configuration to run an exec process.
func (cli *Client) ContainerExecCreate(config types.ExecConfig) (types.ContainerExecCreateResponse, error) {
	var response types.ContainerExecCreateResponse
	resp, err := cli.post("/containers/"+config.Container+"/exec", nil, config, nil)
	if err != nil {
		return response, err
	}
	defer ensureReaderClosed(resp)
	err = json.NewDecoder(resp.body).Decode(&response)
	return response, err
}

// ContainerExecStart starts an exec process already create in the docker host.
func (cli *Client) ContainerExecStart(execID string, config types.ExecStartCheck) error {
	resp, err := cli.post("/exec/"+execID+"/start", nil, config, nil)
	ensureReaderClosed(resp)
	return err
}

// ContainerExecAttach attaches a connection to an exec process in the server.
// It returns a types.HijackedConnection with the hijacked connection
// and the a reader to get output. It's up to the called to close
// the hijacked connection by calling types.HijackedResponse.Close.
func (cli *Client) ContainerExecAttach(execID string, config types.ExecConfig) (types.HijackedResponse, error) {
	headers := map[string][]string{"Content-Type": {"application/json"}}
	return cli.postHijacked("/exec/"+execID+"/start", nil, config, headers)
}

// ContainerExecInspect returns information about a specific exec process on the docker host.
func (cli *Client) ContainerExecInspect(execID string) (types.ContainerExecInspect, error) {
	var response types.ContainerExecInspect
	resp, err := cli.get("/exec/"+execID+"/json", nil, nil)
	if err != nil {
		return response, err
	}
	defer ensureReaderClosed(resp)

	err = json.NewDecoder(resp.body).Decode(&response)
	return response, err
}
