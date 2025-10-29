package network

import (
	"encoding/json"
	"maps"
	"net"
	"net/netip"
	"slices"
)

type MACAddress net.HardwareAddr

func (m MACAddress) String() string {
	return net.HardwareAddr(m).String()
}

func (m MACAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m *MACAddress) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	addr, err := net.ParseMAC(s)
	if err != nil {
		return err
	}
	*m = MACAddress(addr)
	return nil
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configuration data
	IPAMConfig *EndpointIPAMConfig
	Links      []string
	Aliases    []string // Aliases holds the list of extra, user-specified DNS names for this endpoint.
	DriverOpts map[string]string

	// GwPriority determines which endpoint will provide the default gateway
	// for the container. The endpoint with the highest priority will be used.
	// If multiple endpoints have the same priority, they are lexicographically
	// sorted based on their network name, and the one that sorts first is picked.
	GwPriority int

	// Operational data

	NetworkID  string
	EndpointID string
	Gateway    netip.Addr
	IPAddress  netip.Addr

	// MacAddress may be used to specify a MAC address when the container is created.
	// Once the container is running, it becomes operational data (it may contain a
	// generated address).
    MacAddress          MACAddress `json:",omitempty"`
	IPPrefixLen         int
	IPv6Gateway         netip.Addr
	GlobalIPv6Address   netip.Addr
	GlobalIPv6PrefixLen int
	// DNSNames holds all the (non fully qualified) DNS names associated to this
	// endpoint. The first entry is used to generate PTR records.
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
	IPv4Address  netip.Addr   `json:",omitempty"`
	IPv6Address  netip.Addr   `json:",omitempty"`
	LinkLocalIPs []netip.Addr `json:",omitempty"`
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
