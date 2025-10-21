package container

import (
	"net/netip"

	"github.com/moby/moby/api/types/network"
)

// CreateRequest is the request message sent to the server for container
// create calls. It is a config wrapper that holds the container [Config]
// (portable) and the corresponding [HostConfig] (non-portable) and
// [network.NetworkingConfig].
type CreateRequest struct {
	*Config
	HostConfig       *HostConfig             `json:"HostConfig,omitempty"`
	NetworkingConfig NetworkingAttachOptions `json:"NetworkingConfig,omitempty"`
}

// NetworkingAttachOptions represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingAttachOptions struct {
	// TODO(thaJeztah): now could be an opportunity to add a []slice instead of / in addition to the map (to keep things sorted)
	EndpointsConfig map[string]*EndpointSettings // Endpoint configs for each connecting network
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configuration data
	IPAMConfig *network.EndpointIPAMConfig
	Links      []string          `json:",omitempty"` // TODO(thaJeztah): could be more structured for the request (containerID, alias)
	Aliases    []string          `json:",omitempty"` // Aliases holds the list of extra, user-specified DNS names for this endpoint.
	DriverOpts map[string]string `json:",omitempty"`

	// GwPriority determines which endpoint will provide the default gateway
	// for the container. The endpoint with the highest priority will be used.
	// If multiple endpoints have the same priority, they are lexicographically
	// sorted based on their network name, and the one that sorts first is picked.
	GwPriority int `json:",omitempty"`

	NetworkID  string     `json:",omitempty"`
	EndpointID string     `json:",omitempty"`
	Gateway    netip.Addr `json:",omitempty"`
	IPAddress  netip.Addr `json:",omitempty"`

	// TODO(thaJeztah): any of these fields NOT user-configurable? So not belonging in the "create (or "update") request"?

	// MacAddress is the desired MAC address.
	MacAddress          string     `json:",omitempty"`
	IPPrefixLen         int        `json:",omitempty"`
	IPv6Gateway         netip.Addr `json:",omitempty"`
	GlobalIPv6Address   netip.Addr `json:",omitempty"`
	GlobalIPv6PrefixLen int
	// DNSNames holds all the (non fully qualified) DNS names associated to this
	// endpoint. The first entry is used to generate PTR records.
	//
	// TODO(thaJeztah): use this instead of Aliases and/or rename to "ExtraDNSNames"?
	DNSNames []string `json:",omitempty"`
}
