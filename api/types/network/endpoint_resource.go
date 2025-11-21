// Code generated from OpenAPI definition. DO NOT EDIT.

package network

import (
	"net/netip"
)

// EndpointResource contains network resources allocated and used for a container in a network.
type EndpointResource struct {
	//
	// Example: container_1
	Name string `json:"Name,omitempty"`

	//
	// Example: 628cadb8bcb92de107b2a1e516cbffe463e321f548feb37697cce00ad694f21a
	EndpointID string `json:"EndpointID,omitempty"`

	//
	// Example: 02:42:ac:13:00:02
	MacAddress HardwareAddr `json:"MacAddress,omitempty"`

	//
	// Example: 172.19.0.2/16
	IPv4Address netip.Prefix `json:"IPv4Address,omitempty"`

	//
	IPv6Address netip.Prefix `json:"IPv6Address,omitempty"`
}
