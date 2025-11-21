// Code generated from OpenAPI definition. DO NOT EDIT.

package network

// SubnetStatus
type SubnetStatus struct {
	// Number of IP addresses in the subnet that are in use or reserved and are therefore unavailable for allocation, saturating at 2<sup>64</sup> - 1.
	//
	IPsInUse uint64 `json:"IPsInUse,omitempty"`

	// Number of IP addresses within the network's IPRange for the subnet that are available for allocation, saturating at 2<sup>64</sup> - 1.
	//
	DynamicIPsAvailable uint64 `json:"DynamicIPsAvailable,omitempty"`
}
