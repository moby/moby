package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
)

// ContainerExecCreate creates a new exec configuration to run an exec process.
func (cli *Client) ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error) {
	var response types.IDResponse

	// Make sure we negotiated (if the client is configured to do so),
	// as code below contains API-version specific handling of options.
	//
	// Normally, version-negotiation (if enabled) would not happen until
	// the API request is made.
	if err := cli.checkVersion(ctx); err != nil {
		return response, err
	}

	if err := cli.NewVersionError(ctx, "1.25", "env"); len(config.Env) != 0 && err != nil {
		return response, err
	}
	if versions.LessThan(cli.ClientVersion(), "1.42") {
		config.ConsoleSize = nil
	}

	resp, err := cli.post(ctx, "/containers/"+container+"/exec", nil, config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}
	err = json.NewDecoder(resp.body).Decode(&response)
	return response, err
}

// ContainerExecStart starts an exec process already created in the docker host.
func (cli *Client) ContainerExecStart(ctx context.Context, execID string, config types.ExecStartCheck) error {
	if versions.LessThan(cli.ClientVersion(), "1.42") {
		config.ConsoleSize = nil
	}
	resp, err := cli.post(ctx, "/exec/"+execID+"/start", nil, config, nil)
	ensureReaderClosed(resp)
	return err
}

// ContainerExecAttach attaches a connection to an exec process in the server.
// It returns a types.HijackedConnection with the hijacked connection
// and the a reader to get output. It's up to the called to close
// the hijacked connection by calling types.HijackedResponse.Close.
func (cli *Client) ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error) {
	if versions.LessThan(cli.ClientVersion(), "1.42") {
		config.ConsoleSize = nil
	}
	return cli.postHijacked(ctx, "/exec/"+execID+"/start", nil, config, http.Header{
		"Content-Type": {"application/json"},
	})
}

// ContainerExecInspect returns information about a specific exec process on the docker host.
func (cli *Client) ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error) {
	var response types.ContainerExecInspect
	resp, err := cli.get(ctx, "/exec/"+execID+"/json", nil, nil)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)
	ensureReaderClosed(resp)
	return response, err
}
