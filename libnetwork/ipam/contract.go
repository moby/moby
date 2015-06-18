// Package ipam that specifies the contract the IPAM plugin need to satisfy,
// decoupling IPAM interface and implementation.
package ipam

import (
	"errors"
	"net"
)

/**************
 * IPAM Errors
 **************/

// ErrIpamNotAvailable is returned when the plugin prviding the IPAM service is not available
var (
	ErrInvalidIpamService       = errors.New("Invalid IPAM Service")
	ErrInvalidIpamConfigService = errors.New("Invalid IPAM Config Service")
	ErrIpamNotAvailable         = errors.New("IPAM Service not available")
	ErrIpamInternalError        = errors.New("IPAM Internal Error")
	ErrInvalidAddressSpace      = errors.New("Invalid Address Space")
	ErrInvalidSubnet            = errors.New("Invalid Subnet")
	ErrInvalidRequest           = errors.New("Invalid Request")
	ErrSubnetNotFound           = errors.New("Subnet not found")
	ErrOverlapSubnet            = errors.New("Subnet overlaps with existing subnet on this address space")
	ErrNoAvailableSubnet        = errors.New("No available subnet")
	ErrNoAvailableIPs           = errors.New("No available addresses on subnet")
	ErrIPAlreadyAllocated       = errors.New("Address already in use")
	ErrIPOutOfRange             = errors.New("Requested address is out of range")
	ErrSubnetAlreadyRegistered  = errors.New("Subnet already registered on this address space")
	ErrBadSubnet                = errors.New("Address space does not contain specified subnet")
)

// AddressSpace identifies a unique pool of network addresses
type AddressSpace string

/*******************************
 * IPAM Configuration Interface
 *******************************/

// Config represents the interface the IPAM service plugins must implement
// in order to allow injection/modification of IPAM database.
// Common key is a addressspace
type Config interface {
	// AddSubnet adds a subnet to the specified address space
	AddSubnet(AddressSpace, *SubnetInfo) error
	// RemoveSubnet removes a subnet from the specified address space
	RemoveSubnet(AddressSpace, *net.IPNet) error
	// AddVendorInfo adds Vendor specific data
	AddVendorInfo([]byte) error
}

// SubnetInfo contains the information subnet hosts need in order to communicate
type SubnetInfo struct {
	Subnet     *net.IPNet
	Gateway    net.IP
	OpaqueData []byte // Vendor specific
}

/*************************
 * IPAM Service Interface
 *************************/

// IPAM defines the interface that needs to be implemented by IPAM service plugin
// Common key is a unique address space identifier
type IPAM interface {
	// Request address from the specified address space
	Request(AddressSpace, *AddressRequest) (*AddressResponse, error)
	// Separate API for IPv6
	RequestV6(AddressSpace, *AddressRequest) (*AddressResponse, error)
	// Release the address from the specified address space
	Release(AddressSpace, net.IP)
}

// AddressRequest encloses the information a client
// needs to pass to IPAM when requesting an address
type AddressRequest struct {
	Subnet     net.IPNet // Preferred subnet pool (Optional)
	Address    net.IP    // Preferred address (Optional)
	Endpoint   string    // For static IP mapping (Optional)
	OpaqueData []byte    // Vendor specific request data
}

// Validate runs syntactic validation on this AddressRequest object
func (req *AddressRequest) Validate() error {
	var byteArray []byte = req.Address

	// Check preferred address
	if byteArray != nil && (&req.Subnet == nil || !req.Subnet.Contains(req.Address)) {
		return ErrInvalidRequest
	}

	return nil
}

// AddressResponse represents the IPAM service's
// response to an address request
type AddressResponse struct {
	Address net.IP
	Subnet  SubnetInfo
}
