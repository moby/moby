package daemon

import (
	"strings"

	"github.com/moby/moby/v2/daemon/libnetwork"
)

// getEndpointInNetwork returns the container's endpoint to the provided network.
func getEndpointInNetwork(name string, n *libnetwork.Network) (*libnetwork.Endpoint, error) {
	endpointName := strings.TrimPrefix(name, "/")
	return n.EndpointByName(endpointName)
}
