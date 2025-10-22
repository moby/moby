package client

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/swarm"
)

// PingOptions holds options for [client.Ping].
type PingOptions struct {
	// Add future optional parameters here
}

// PingResult holds the result of a [Client.Ping] API call.
type PingResult struct {
	APIVersion     string
	OSType         string
	Experimental   bool
	BuilderVersion build.BuilderVersion

	// SwarmStatus provides information about the current swarm status of the
	// engine, obtained from the "Swarm" header in the API response.
	//
	// It can be a nil struct if the API version does not provide this header
	// in the ping response, or if an error occurred, in which case the client
	// should use other ways to get the current swarm status, such as the /swarm
	// endpoint.
	SwarmStatus *SwarmStatus
}

// SwarmStatus provides information about the current swarm status and role,
// obtained from the "Swarm" header in the API response.
type SwarmStatus struct {
	// NodeState represents the state of the node.
	NodeState swarm.LocalNodeState

	// ControlAvailable indicates if the node is a swarm manager.
	ControlAvailable bool
}

// Ping pings the server and returns the value of the "Docker-Experimental",
// "Builder-Version", "OS-Type" & "API-Version" headers. It attempts to use
// a HEAD request on the endpoint, but falls back to GET if HEAD is not supported
// by the daemon. It ignores internal server errors returned by the API, which
// may be returned if the daemon is in an unhealthy state, but returns errors
// for other non-success status codes, failing to connect to the API, or failing
// to parse the API response.
func (cli *Client) Ping(ctx context.Context, options PingOptions) (PingResult, error) {
	// Using cli.buildRequest() + cli.doRequest() instead of cli.sendRequest()
	// because ping requests are used during API version negotiation, so we want
	// to hit the non-versioned /_ping endpoint, not /v1.xx/_ping
	req, err := cli.buildRequest(ctx, http.MethodHead, path.Join(cli.basePath, "/_ping"), nil, nil)
	if err != nil {
		return PingResult{}, err
	}
	resp, err := cli.doRequest(req)
	defer ensureReaderClosed(resp)
	if err == nil && resp.StatusCode == http.StatusOK {
		// Fast-path; successfully connected using a HEAD request and
		// we got a "OK" (200) status. For non-200 status-codes, we fall
		// back to doing a GET request, as a HEAD request won't have a
		// response-body to get error details from.
		return newPingResult(resp), nil
	}

	// HEAD failed or returned a non-OK status; fallback to GET.
	req.Method = http.MethodGet
	resp, err = cli.doRequest(req)
	defer ensureReaderClosed(resp)
	if err != nil {
		// Failed to connect.
		return PingResult{}, err
	}

	// GET request succeeded but may have returned a non-200 status.
	// Return a Ping response, together with any error returned by
	// the API server.
	return newPingResult(resp), checkResponseErr(resp)
}

func newPingResult(resp *http.Response) PingResult {
	if resp == nil {
		return PingResult{}
	}
	var swarmStatus *SwarmStatus
	if si := resp.Header.Get("Swarm"); si != "" {
		state, role, _ := strings.Cut(si, "/")
		swarmStatus = &SwarmStatus{
			NodeState:        swarm.LocalNodeState(state),
			ControlAvailable: role == "manager",
		}
	}

	return PingResult{
		APIVersion:     resp.Header.Get("Api-Version"),
		OSType:         resp.Header.Get("Ostype"),
		Experimental:   resp.Header.Get("Docker-Experimental") == "true",
		BuilderVersion: build.BuilderVersion(resp.Header.Get("Builder-Version")),
		SwarmStatus:    swarmStatus,
	}
}
