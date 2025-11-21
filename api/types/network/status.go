// Code generated from OpenAPI definition. DO NOT EDIT.

package network

// Status provides runtime information about the network such as the number of allocated IPs.
type Status struct {
	//
	// Required: true
	IPAM IPAMStatus `json:"IPAM"`
}
