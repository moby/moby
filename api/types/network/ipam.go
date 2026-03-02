package network

import (
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

type SubnetStatuses = map[netip.Prefix]SubnetStatus
