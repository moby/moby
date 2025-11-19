// Code generated from OpenAPI definition. DO NOT EDIT.

package network

import (
	"net/netip"
)

// PeerInfo represents one peer of an overlay network.
type PeerInfo struct {
	// ID of the peer-node in the Swarm cluster.
	// Example: 6869d7c1732b
	Name string `json:"Name,omitempty"`

	// IP-address of the peer-node in the Swarm cluster.
	// Example: 10.133.77.91
	IP netip.Addr `json:"IP,omitempty"`
}
