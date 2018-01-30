package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/exec"
	"golang.org/x/net/context"
)

// ContainerExecCreate creates a new exec configuration to run an exec process.
func (cli *Client) ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error) {
	var response types.IDResponse

	if err := cli.NewVersionError("1.25", "env"); len(config.Env) != 0 && err != nil {
		return response, err
	}

	resp, err := cli.post(ctx, "/containers/"+container+"/exec", nil, config, nil)
	if err != nil {
		return response, err
	}
	err = json.NewDecoder(resp.body).Decode(&response)
	ensureReaderClosed(resp)
	return response, err
}

// ContainerExecStart starts an exec process already created in the docker host.
func (cli *Client) ContainerExecStart(ctx context.Context, execID string, config types.ExecStartCheck) error {
	resp, err := cli.post(ctx, "/exec/"+execID+"/start", nil, config, nil)
	ensureReaderClosed(resp)
	return err
}

// ContainerExecAttach attaches a connection to an exec process in the server.
// It returns a types.HijackedConnection with the hijacked connection
// and the a reader to get output. It's up to the called to close
// the hijacked connection by calling types.HijackedResponse.Close.
func (cli *Client) ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error) {
	headers := map[string][]string{"Content-Type": {"application/json"}}
	return cli.postHijacked(ctx, "/exec/"+execID+"/start", nil, config, headers)
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

// ContainerExecWait waits until the specified exec is in a certain state
// indicated by the given condition, either "running" (default) or "exit".
func (cli *Client) ContainerExecWait(ctx context.Context, execID string, cond exec.WaitCondition) (<-chan exec.ExecWaitOKBody, <-chan error) {
	var (
		resultC = make(chan exec.ExecWaitOKBody)
		errC    = make(chan error, 1)
		query   = url.Values{}
	)

	query.Set("condition", string(cond))

	resp, err := cli.post(ctx, "/exec/"+execID+"/wait", query, nil, nil)
	if err != nil {
		defer ensureReaderClosed(resp)
		errC <- err
		return resultC, errC
	}

	go func() {
		defer ensureReaderClosed(resp)
		var res exec.ExecWaitOKBody
		if err := json.NewDecoder(resp.body).Decode(&res); err != nil {
			errC <- err
			return
		}

		resultC <- res
	}()

	return resultC, errC
}
