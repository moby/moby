package network

import (
	"maps"
	"slices"
)

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configurations
	IPAMConfig *EndpointIPAMConfig
	Links      []string
	Aliases    []string // Aliases holds the list of extra, user-specified DNS names for this endpoint.
	// MacAddress may be used to specify a MAC address when the container is created.
	// Once the container is running, it becomes operational data (it may contain a
	// generated address).
	MacAddress string
	DriverOpts map[string]string

	// GwPriority determines which endpoint will provide the default gateway
	// for the container. The endpoint with the highest priority will be used.
	// If multiple endpoints have the same priority, they are lexicographically
	// sorted based on their network name, and the one that sorts first is picked.
	GwPriority int
	// Operational data
	NetworkID           string
	EndpointID          string
	Gateway             string
	IPAddress           string
	IPPrefixLen         int
	IPv6Gateway         string
	GlobalIPv6Address   string
	GlobalIPv6PrefixLen int
	// DNSNames holds all the (non fully qualified) DNS names associated to this endpoint. First entry is used to
	// generate PTR records.
	DNSNames []string
}

// Copy makes a deep copy of `EndpointSettings`
func (es *EndpointSettings) Copy() *EndpointSettings {
	if es == nil {
		return nil
	}

	epCopy := *es
	epCopy.IPAMConfig = es.IPAMConfig.Copy()
	epCopy.Links = slices.Clone(es.Links)
	epCopy.Aliases = slices.Clone(es.Aliases)
	epCopy.DNSNames = slices.Clone(es.DNSNames)
	epCopy.DriverOpts = maps.Clone(es.DriverOpts)

	return &epCopy
}

// EndpointIPAMConfig represents IPAM configurations for the endpoint
type EndpointIPAMConfig struct {
	IPv4Address  string   `json:",omitempty"`
	IPv6Address  string   `json:",omitempty"`
	LinkLocalIPs []string `json:",omitempty"`
}

// Copy makes a copy of the endpoint ipam config
func (cfg *EndpointIPAMConfig) Copy() *EndpointIPAMConfig {
	if cfg == nil {
		return nil
	}
	cfgCopy := *cfg
	cfgCopy.LinkLocalIPs = slices.Clone(cfg.LinkLocalIPs)
	return &cfgCopy
}
