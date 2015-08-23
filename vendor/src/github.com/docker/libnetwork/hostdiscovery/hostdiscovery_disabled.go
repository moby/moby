// +build !libnetwork_discovery

package hostdiscovery

import (
	"net"

	"github.com/docker/libnetwork/config"
)

type hostDiscovery struct{}

// NewHostDiscovery function creates a host discovery object
func NewHostDiscovery() HostDiscovery {
	return &hostDiscovery{}
}

func (h *hostDiscovery) StartDiscovery(cfg *config.ClusterCfg, joinCallback JoinCallback, leaveCallback LeaveCallback) error {
	return nil
}

func (h *hostDiscovery) StopDiscovery() error {
	return nil
}

func (h *hostDiscovery) Fetch() ([]net.IP, error) {
	return []net.IP{}, nil
}
