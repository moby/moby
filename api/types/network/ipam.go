package network

import (
	"encoding/json"
	"net/netip"
)

// IPAM represents IP Address Management
type IPAM struct {
	Driver  string
	Options map[string]string // Per network IPAM driver options
	Config  []IPAMConfig
}

// IPAMConfig represents IPAM configurations
type IPAMConfig struct {
	Subnet     netip.Prefix          `json:"Subnet,omitzero"`
	IPRange    netip.Prefix          `json:"IPRange,omitzero"`
	Gateway    netip.Addr            `json:"Gateway,omitzero"`
	AuxAddress map[string]netip.Addr `json:"AuxiliaryAddresses,omitempty"`
}

// UnmarshalJSON implements [json.Unmarshaler]. It provides lenient parsing for
// Gateway and AuxiliaryAddresses address fields: older Docker daemons may store
// gateway addresses with a CIDR prefix notation (e.g. "fd05:d0ca:2::1/112"),
// which [netip.Addr.UnmarshalText] rejects. To maintain backward compatibility
// when talking to such daemons, the prefix length is silently stripped.
func (c *IPAMConfig) UnmarshalJSON(data []byte) error {
	// Use a shadow type with string fields for the address values so we can
	// apply lenient parsing ourselves.
	var raw struct {
		Subnet     netip.Prefix          `json:"Subnet"`
		IPRange    netip.Prefix          `json:"IPRange"`
		Gateway    string                `json:"Gateway"`
		AuxAddress map[string]string     `json:"AuxiliaryAddresses"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Subnet = raw.Subnet
	c.IPRange = raw.IPRange
	c.Gateway = parseAddrLenient(raw.Gateway)

	if len(raw.AuxAddress) > 0 {
		c.AuxAddress = make(map[string]netip.Addr, len(raw.AuxAddress))
		for k, v := range raw.AuxAddress {
			c.AuxAddress[k] = parseAddrLenient(v)
		}
	}
	return nil
}

// parseAddrLenient parses an IP address string. If the string contains a
// CIDR prefix (e.g. "192.168.1.1/24"), the prefix length is stripped and the
// host address is returned. This provides backwards compatibility with Docker
// daemons that may have stored gateway addresses with prefix notation.
//
// An empty or invalid string returns the zero [netip.Addr].
func parseAddrLenient(s string) netip.Addr {
	if s == "" {
		return netip.Addr{}
	}
	// Try plain address first (the expected, canonical format).
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap()
	}
	// Fall back: try CIDR notation for older Docker daemon responses.
	if prefix, err := netip.ParsePrefix(s); err == nil {
		return prefix.Addr().Unmap()
	}
	return netip.Addr{}
}

type SubnetStatuses = map[netip.Prefix]SubnetStatus
