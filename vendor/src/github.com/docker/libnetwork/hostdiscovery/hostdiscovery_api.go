package hostdiscovery

import (
	"net"

	"github.com/docker/libnetwork/config"
)

// JoinCallback provides a callback event for new node joining the cluster
type JoinCallback func(entries []net.IP)

// LeaveCallback provides a callback event for node leaving the cluster
type LeaveCallback func(entries []net.IP)

// HostDiscovery primary interface
type HostDiscovery interface {
	// StartDiscovery initiates the discovery process and provides appropriate callbacks
	StartDiscovery(*config.ClusterCfg, JoinCallback, LeaveCallback) error
	// StopDiscovery stops the discovery perocess
	StopDiscovery() error
	// Fetch returns a list of host IPs that are currently discovered
	Fetch() ([]net.IP, error)
}
