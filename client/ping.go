package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

// Ping pings the server and returns the value of the "Docker-Experimental",
// "Builder-Version", "OS-Type" & "API-Version" headers. It attempts to use
// a HEAD request on the endpoint, but falls back to GET if HEAD is not supported
// by the daemon.
func (cli *Client) Ping(ctx context.Context) (types.Ping, error) {
	// Ping requests are used during API version negotiation, so we want to
	// hit the non-versioned /_ping endpoint, not /v1.xx/_ping
	unversioned := versionedClient{cli: cli, version: ""}
	serverResp, err := unversioned.head(ctx, "/_ping", nil, nil)
	ensureReaderClosed(serverResp) // We're only interested in the headers.
	switch serverResp.statusCode {
	case http.StatusOK, http.StatusInternalServerError:
		// Server handled the request, so parse the response
		return parsePingResponse(serverResp.header), err
	}
	// We only want to fall back to GET if the daemon is reachable but does
	// not support HEAD /_ping requests. The client converts status codes
	// outside of the 2xx and 3xx ranges into different kinds of errors,
	// which makes it awkward and error-prone to differentiate "errors
	// returned by the daemon" from "errors making the request" by testing
	// only the error value. There is an easy tell, however:
	// serverResp.statusCode is set to a positive value iff the HTTP client
	// successfully received and parsed the server response, therefore in
	// such cases any returned error must be an error returned by the
	// daemon.
	if err != nil && serverResp.statusCode <= 0 {
		return types.Ping{}, err
	}

	serverResp, err = unversioned.get(ctx, "/_ping", nil, nil)
	ensureReaderClosed(serverResp)
	return parsePingResponse(serverResp.header), err
}

func parsePingResponse(header http.Header) types.Ping {
	var swarmStatus *swarm.Status
	if si := header.Get("Swarm"); si != "" {
		parts := strings.SplitN(si, "/", 2)
		swarmStatus = &swarm.Status{
			NodeState:        swarm.LocalNodeState(parts[0]),
			ControlAvailable: len(parts) == 2 && parts[1] == "manager",
		}
	}
	return types.Ping{
		APIVersion:     header.Get("API-Version"),
		OSType:         header.Get("OSType"),
		Experimental:   header.Get("Docker-Experimental") == "true",
		BuilderVersion: types.BuilderVersion(header.Get("Builder-Version")),
		SwarmStatus:    swarmStatus,
	}
}
