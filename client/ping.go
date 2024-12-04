package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// Ping pings the server and returns the value of the "Docker-Experimental",
// "Builder-Version", "OS-Type" & "API-Version" headers. It attempts to use
// a HEAD request on the endpoint, but falls back to GET if HEAD is not supported
// by the daemon. It ignores internal server errors returned by the API, which
// may be returned if the daemon is in an unhealthy state, but returns errors
// for other non-success status codes, failing to connect to the API, or failing
// to parse the API response.
func (cli *Client) Ping(ctx context.Context, requestCapabilities bool) (types.Ping, error) {
	var ping types.Ping

	// If not interested in engine capabilities, do a HEAD request
	if !requestCapabilities {
		// Using cli.buildRequest() + cli.doRequest() instead of cli.sendRequest()
		// because ping requests are used during API version negotiation, so we want
		// to hit the non-versioned /_ping endpoint, not /v1.xx/_ping
		req, err := cli.buildRequest(ctx, http.MethodHead, path.Join(cli.basePath, "/_ping"), nil, nil)
		if err != nil {
			return ping, err
		}
		serverResp, err := cli.doRequest(req)
		if err == nil {
			defer ensureReaderClosed(serverResp)
			switch serverResp.statusCode {
			case http.StatusOK, http.StatusInternalServerError:
				// Server handled the request, so parse the response
				return parsePingResponse(cli, serverResp)
			}
		} else if IsErrConnectionFailed(err) {
			return ping, err
		}
	}

	// if the HEAD request failed – or we're requesting capabilities –
	// do a GET request
	query := url.Values{}
	if requestCapabilities {
		query.Set("features", "v1")
	}

	serverResp, err := cli.get(ctx, path.Join(cli.basePath, "/_ping"), query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return ping, err
	}
	return parsePingResponse(cli, serverResp)
}

func parsePingResponse(cli *Client, resp serverResponse) (types.Ping, error) {
	var ping types.Ping
	if resp.header == nil {
		err := cli.checkResponseErr(resp)
		return ping, errdefs.FromStatusCode(err, resp.statusCode)
	}
	ping.APIVersion = resp.header.Get("API-Version")
	ping.OSType = resp.header.Get("OSType")
	if resp.header.Get("Docker-Experimental") == "true" {
		ping.Experimental = true
	}
	if bv := resp.header.Get("Builder-Version"); bv != "" {
		ping.BuilderVersion = types.BuilderVersion(bv)
	}
	if si := resp.header.Get("Swarm"); si != "" {
		state, role, _ := strings.Cut(si, "/")
		ping.SwarmStatus = &swarm.Status{
			NodeState:        swarm.LocalNodeState(state),
			ControlAvailable: role == "manager",
		}
	}

	err := cli.checkResponseErr(resp)
	// if  200 <= statuscode < 400, this will return nil and not read the body
	if err != nil {
		return ping, errdefs.FromStatusCode(err, resp.statusCode)
	}

	// check the body for capabilities if it's not nil and the response was not an error
	if resp.body != nil {
		capabilities, err := parseCapabilitiesFromBody(resp.body)
		if err != nil {
			return ping, fmt.Errorf("failed to parse ping body: %w", err)
		}
		ping.Capabilities = capabilities
	}
	return ping, nil
}

func parseCapabilitiesFromBody(pingBody io.Reader) (*system.Capabilities, error) {
	content, err := io.ReadAll(pingBody)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 || string(content) == "OK" {
		return nil, nil
	}
	var capabilities system.Capabilities
	err = json.Unmarshal(content, &capabilities)
	if err != nil {
		return nil, errors.Errorf("expected capabilities, found '%s'", string(content))
	}
	return &capabilities, nil
}
