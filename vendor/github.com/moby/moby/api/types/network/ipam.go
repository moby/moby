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
	Subnet     netip.Prefix          `json:",omitempty"`
	IPRange    netip.Prefix          `json:",omitempty"`
	Gateway    netip.Addr            `json:",omitempty"`
	AuxAddress map[string]netip.Addr `json:"AuxiliaryAddresses,omitempty"`
}

type SubnetStatuses = map[netip.Prefix]SubnetStatus
