package daemon

import (
	"strings"

	"github.com/docker/docker/libnetwork"
)

// getEndpointInNetwork returns the container's endpoint to the provided network.
func getEndpointInNetwork(name string, n libnetwork.Network) (*libnetwork.Endpoint, error) {
	endpointName := strings.TrimPrefix(name, "/")
	return n.EndpointByName(endpointName)
}
