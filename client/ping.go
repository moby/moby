package client // import "github.com/docker/docker/client"

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
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
func (cli *Client) Ping(ctx context.Context) (types.Ping, error) {
	var ping types.Ping

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

	// HEAD failed; fallback to GET.
	req.Method = http.MethodGet
	serverResp, err = cli.doRequest(req)
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
	if engineFeatureHeaders := resp.header.Values("Engine-Features"); len(engineFeatureHeaders) > 0 {
		featuresMap, err := parseEngineFeaturesHeader(engineFeatureHeaders)
		if err != nil {
			return ping, fmt.Errorf("failed to parse Engine-Features header: %w", err)
		}
		ping.EngineFeatures = featuresMap
	}
	err := cli.checkResponseErr(resp)
	return ping, errdefs.FromStatusCode(err, resp.statusCode)
}

// Prevent a malicious engine from blowing up clients by sending
// a huge number of engine features.
const maxNumEngineFeatures = 100

func parseEngineFeaturesHeader(headers []string) (map[string]bool, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	var totalNumFeatures int
	featuresMap := make(map[string]bool)
	for _, h := range headers {
		if h == "" {
			continue
		}

		totalNumFeatures += numFeatures(h)
		if totalNumFeatures > maxNumEngineFeatures {
			return nil, errors.Errorf("too many features: expected max %d, found %d", maxNumEngineFeatures, totalNumFeatures)
		}

		features := strings.Split(h, ",")
		for _, featureValue := range features {
			k, v, found := strings.Cut(featureValue, "=")
			if !found {
				return nil, fmt.Errorf("feature '%s' is missing '='", featureValue)
			}
			if _, alreadyExists := featuresMap[k]; alreadyExists {
				return nil, fmt.Errorf("duplicate feature '%s'", k)
			}
			// TODO(laurazard): unnecessary, would be caught by strconv.ParseBool, maybe we should remove it
			if strings.Contains(v, "=") {
				return nil, fmt.Errorf("feature '%s' has too many '='", featureValue)
			}
			enabled, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("feature '%s': - %w", featureValue, err)
			}
			featuresMap[k] = enabled
		}
	}

	return featuresMap, nil
}

func numFeatures(header string) int {
	return strings.Count(header, ",") + 1
}
