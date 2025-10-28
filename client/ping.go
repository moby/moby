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
	// NegotiateAPIVersion queries the API and updates the version to match the API
	// version. NegotiateAPIVersion downgrades the client's API version to match the
	// APIVersion if the ping version is lower than the default version. If the API
	// version reported by the server is higher than the maximum version supported
	// by the client, it uses the client's maximum version.
	//
	// If a manual override is in place, either through the "DOCKER_API_VERSION"
	// ([EnvOverrideAPIVersion]) environment variable, or if the client is initialized
	// with a fixed version ([WithVersion]), no negotiation is performed.
	//
	// If the API server's ping response does not contain an API version, or if the
	// client did not get a successful ping response, it assumes it is connected with
	// an old daemon that does not support API version negotiation, in which case it
	// downgrades to the lowest supported API version.
	NegotiateAPIVersion bool

	// ForceNegotiate forces the client to re-negotiate the API version, even if
	// API-version negotiation already happened. This option cannot be
	// used if the client is configured with a fixed version using (using
	// [WithVersion] or [WithVersionFromEnv]).
	//
	// This option has no effect if NegotiateAPIVersion is not set.
	ForceNegotiate bool
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
	if cli.manualOverride {
		return cli.ping(ctx)
	}
	if !options.NegotiateAPIVersion && !cli.negotiateVersion {
		return cli.ping(ctx)
	}

	// Ensure exclusive write access to version and negotiated fields
	cli.negotiateLock.Lock()
	defer cli.negotiateLock.Unlock()

	ping, err := cli.ping(ctx)
	if err != nil {
		return cli.ping(ctx)
	}

	if cli.negotiated.Load() && !options.ForceNegotiate {
		return ping, nil
	}

	return ping, cli.negotiateAPIVersion(ping.APIVersion)
}

func (cli *Client) ping(ctx context.Context) (PingResult, error) {
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
