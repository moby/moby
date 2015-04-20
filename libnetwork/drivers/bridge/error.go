package bridge

import (
	"errors"
	"fmt"
	"net"
)

var (
	// ErrConfigExists error is returned when driver already has a config applied.
	ErrConfigExists = errors.New("configuration already exists, simplebridge configuration can be applied only once")

	// ErrInvalidConfig error is returned when a network is created on a driver without valid config.
	ErrInvalidConfig = errors.New("trying to create a network on a driver without valid config")

	// ErrNetworkExists error is returned when a network already exists and another network is created.
	ErrNetworkExists = errors.New("network already exists, simplebridge can only have one network")

	// ErrIfaceName error is returned when a new name could not be generated.
	ErrIfaceName = errors.New("Failed to find name for new interface")

	// ErrNoIPAddr error is returned when bridge has no IPv4 address configured.
	ErrNoIPAddr = errors.New("Bridge has no IPv4 address configured")
)

// ActiveEndpointsError is returned when there are
// already active endpoints in the network being deleted.
type ActiveEndpointsError struct {
	nid string
	eid string
}

func (aee *ActiveEndpointsError) Error() string {
	return fmt.Sprintf("Network %s has active endpoint %s", aee.nid, aee.eid)
}

// InvalidNetworkIDError is returned when the passed
// network id for an existing network is not a known id.
type InvalidNetworkIDError string

func (inie InvalidNetworkIDError) Error() string {
	return fmt.Sprintf("invalid network id %s", inie)
}

// InvalidEndpointIDError is returned when the passed
// endpoint id for an existing endpoint is not a known id.
type InvalidEndpointIDError string

func (ieie InvalidEndpointIDError) Error() string {
	return fmt.Sprintf("invalid endpoint id %s", ieie)
}

// NonDefaultBridgeExistError is returned when a non-default
// bridge config is passed but it does not already exist.
type NonDefaultBridgeExistError string

func (ndbee NonDefaultBridgeExistError) Error() string {
	return fmt.Sprintf("bridge device with non default name %s must be created manually", ndbee)
}

// FixedCIDRv4Error is returned when fixed-cidrv4 configuration
// failed.
type FixedCIDRv4Error struct {
	net    *net.IPNet
	subnet *net.IPNet
	err    error
}

func (fcv4 *FixedCIDRv4Error) Error() string {
	return fmt.Sprintf("Setup FixedCIDRv4 failed for subnet %s in %s: %v", fcv4.subnet, fcv4.net, fcv4.err)
}

// FixedCIDRv6Error is returned when fixed-cidrv6 configuration
// failed.
type FixedCIDRv6Error struct {
	net *net.IPNet
	err error
}

func (fcv6 *FixedCIDRv6Error) Error() string {
	return fmt.Sprintf("Setup FixedCIDRv6 failed for subnet %s in %s: %v", fcv6.net, fcv6.net, fcv6.err)
}

type ipForwardCfgError bridgeInterface

func (i *ipForwardCfgError) Error() string {
	return fmt.Sprintf("Unexpected request to enable IP Forwarding for: %v", *i)
}

type ipTableCfgError string

func (name ipTableCfgError) Error() string {
	return fmt.Sprintf("Unexpected request to set IP tables for interface: %s", name)
}

// IPv4AddrRangeError is returned when a valid IP address range couldn't be found.
type IPv4AddrRangeError string

func (name IPv4AddrRangeError) Error() string {
	return fmt.Sprintf("can't find an address range for interface %q", name)
}

// IPv4AddrAddError is returned when IPv4 address could not be added to the bridge.
type IPv4AddrAddError struct {
	ip  *net.IPNet
	err error
}

func (ipv4 *IPv4AddrAddError) Error() string {
	return fmt.Sprintf("Failed to add IPv4 address %s to bridge: %v", ipv4.ip, ipv4.err)
}

// IPv6AddrAddError is returned when IPv6 address could not be added to the bridge.
type IPv6AddrAddError struct {
	ip  *net.IPNet
	err error
}

func (ipv6 *IPv6AddrAddError) Error() string {
	return fmt.Sprintf("Failed to add IPv6 address %s to bridge: %v", ipv6.ip, ipv6.err)
}

// IPv4AddrNoMatchError is returned when the bridge's IPv4 address does not match configured.
type IPv4AddrNoMatchError struct {
	ip    net.IP
	cfgIP net.IP
}

func (ipv4 *IPv4AddrNoMatchError) Error() string {
	return fmt.Sprintf("Bridge IPv4 (%s) does not match requested configuration %s", ipv4.ip, ipv4.cfgIP)
}

// IPv6AddrNoMatchError is returned when the bridge's IPv6 address does not match configured.
type IPv6AddrNoMatchError net.IPNet

func (ipv6 *IPv6AddrNoMatchError) Error() string {
	return fmt.Sprintf("Bridge IPv6 addresses do not match the expected bridge configuration %s", ipv6)
}
