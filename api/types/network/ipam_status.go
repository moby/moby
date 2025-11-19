// Code generated from OpenAPI definition. DO NOT EDIT.

package network

// IPAMStatus
type IPAMStatus struct {
	//
	// Example: {
	//   "172.16.0.0/16": {
	//     "DynamicIPsAvailable": 65533,
	//     "IPsInUse": 3
	//   },
	//   "2001:db8:abcd:0012::0/96": {
	//     "DynamicIPsAvailable": 4294967291,
	//     "IPsInUse": 5
	//   }
	// }
	Subnets SubnetStatuses `json:"Subnets,omitempty"`
}
