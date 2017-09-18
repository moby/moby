package network

import (
	"net"

	networktypes "github.com/docker/docker/api/types/network"
	clustertypes "github.com/docker/docker/daemon/cluster/provider"
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

// LBAttachmentStore stores the load balancer IP address for a network id.
type LBAttachmentStore struct {
	//key: networkd id
	//value: load balancer ip address
	networkToNodeLBIP map[string]net.IP
}

// ResetLBAttachments clears any exsiting load balancer IP to network mapping and
// sets the mapping to the given lbAttachments.
func (lbStore *LBAttachmentStore) ResetLBAttachments(lbAttachments map[string]string) error {
	lbStore.ClearLBAttachments()
	for nid, nodeIP := range lbAttachments {
		ip, _, err := net.ParseCIDR(nodeIP)
		if err != nil {
			lbStore.networkToNodeLBIP = make(map[string]net.IP)
			return errors.Wrapf(err, "Failed to parse load balancer address %s", nodeIP)
		}
		lbStore.networkToNodeLBIP[nid] = ip
	}
	return nil
}

// ClearLBAttachments clears all the mappings of network to load balancer IP Address.
func (lbStore *LBAttachmentStore) ClearLBAttachments() {
	lbStore.networkToNodeLBIP = make(map[string]net.IP)
}

// GetLBIPForNetwork return the load balancer IP address for the given network.
func (lbStore *LBAttachmentStore) GetLBIPForNetwork(networkID string) (net.IP, bool) {
	ip, exists := lbStore.networkToNodeLBIP[networkID]
	return ip, exists
}
