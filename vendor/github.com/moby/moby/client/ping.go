package client

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/swarm"
)

// Ping pings the server and returns the value of the "Docker-Experimental",
// "Builder-Version", "OS-Type" & "API-Version" headers. It attempts to use
// a HEAD request on the endpoint, but falls back to GET if HEAD is not supported
// by the daemon. It ignores internal server errors returned by the API, which
// may be returned if the daemon is in an unhealthy state, but returns errors
// for other non-success status codes, failing to connect to the API, or failing
// to parse the API response.
func (cli *Client) Ping(ctx context.Context) (types.Ping, error) {
	// Using cli.buildRequest() + cli.doRequest() instead of cli.sendRequest()
	// because ping requests are used during API version negotiation, so we want
	// to hit the non-versioned /_ping endpoint, not /v1.xx/_ping
	req, err := cli.buildRequest(ctx, http.MethodHead, path.Join(cli.basePath, "/_ping"), nil, nil)
	if err != nil {
		return types.Ping{}, err
	}
	resp, err := cli.doRequest(req)
	defer ensureReaderClosed(resp)
	if err == nil && resp.StatusCode == http.StatusOK {
		// Fast-path; successfully connected using a HEAD request and
		// we got a "OK" (200) status. For non-200 status-codes, we fall
		// back to doing a GET request, as a HEAD request won't have a
		// response-body to get error details from.
		return newPingResponse(resp), nil
	}

	// HEAD failed or returned a non-OK status; fallback to GET.
	req.Method = http.MethodGet
	resp, err = cli.doRequest(req)
	defer ensureReaderClosed(resp)
	if err != nil {
		// Failed to connect.
		return types.Ping{}, err
	}

	// GET request succeeded but may have returned a non-200 status.
	// Return a Ping response, together with any error returned by
	// the API server.
	return newPingResponse(resp), checkResponseErr(resp)
}

func newPingResponse(resp *http.Response) types.Ping {
	if resp == nil {
		return types.Ping{}
	}
	var swarmStatus *swarm.Status
	if si := resp.Header.Get("Swarm"); si != "" {
		state, role, _ := strings.Cut(si, "/")
		swarmStatus = &swarm.Status{
			NodeState:        swarm.LocalNodeState(state),
			ControlAvailable: role == "manager",
		}
	}

	return types.Ping{
		APIVersion:     resp.Header.Get("Api-Version"),
		OSType:         resp.Header.Get("Ostype"),
		Experimental:   resp.Header.Get("Docker-Experimental") == "true",
		BuilderVersion: build.BuilderVersion(resp.Header.Get("Builder-Version")),
		SwarmStatus:    swarmStatus,
	}
}
