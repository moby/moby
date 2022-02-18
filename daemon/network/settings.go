package network // import "github.com/moby/moby/daemon/network"

import (
	"net"
	"sync"

	networktypes "github.com/moby/moby/api/types/network"
	clustertypes "github.com/moby/moby/daemon/cluster/provider"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

// Settings stores configuration details about the daemon network config
// TODO Windows. Many of these fields can be factored out.,
type Settings struct {
	Bridge                 string
	SandboxID              string
	HairpinMode            bool
	LinkLocalIPv6Address   string
	LinkLocalIPv6PrefixLen int
	Networks               map[string]*EndpointSettings
	Service                *clustertypes.ServiceConfig
	Ports                  nat.PortMap
	SandboxKey             string
	SecondaryIPAddresses   []networktypes.Address
	SecondaryIPv6Addresses []networktypes.Address
	IsAnonymousEndpoint    bool
	HasSwarmEndpoint       bool
}

// EndpointSettings is a package local wrapper for
// networktypes.EndpointSettings which stores Endpoint state that
// needs to be persisted to disk but not exposed in the api.
type EndpointSettings struct {
	*networktypes.EndpointSettings
	IPAMOperational bool
}

// AttachmentStore stores the load balancer IP address for a network id.
type AttachmentStore struct {
	sync.Mutex
	// key: networkd id
	// value: load balancer ip address
	networkToNodeLBIP map[string]net.IP
}

// ResetAttachments clears any existing load balancer IP to network mapping and
// sets the mapping to the given attachments.
func (store *AttachmentStore) ResetAttachments(attachments map[string]string) error {
	store.Lock()
	defer store.Unlock()
	store.clearAttachments()
	for nid, nodeIP := range attachments {
		ip, _, err := net.ParseCIDR(nodeIP)
		if err != nil {
			store.networkToNodeLBIP = make(map[string]net.IP)
			return errors.Wrapf(err, "Failed to parse load balancer address %s", nodeIP)
		}
		store.networkToNodeLBIP[nid] = ip
	}
	return nil
}

// ClearAttachments clears all the mappings of network to load balancer IP Address.
func (store *AttachmentStore) ClearAttachments() {
	store.Lock()
	defer store.Unlock()
	store.clearAttachments()
}

func (store *AttachmentStore) clearAttachments() {
	store.networkToNodeLBIP = make(map[string]net.IP)
}

// GetIPForNetwork return the load balancer IP address for the given network.
func (store *AttachmentStore) GetIPForNetwork(networkID string) (net.IP, bool) {
	store.Lock()
	defer store.Unlock()
	ip, exists := store.networkToNodeLBIP[networkID]
	return ip, exists
}
