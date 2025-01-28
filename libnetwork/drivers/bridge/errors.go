//go:build linux

package bridge

import (
	"fmt"
	"net"
)

// ErrNoIPAddr error is returned when bridge has no IPv4 address configured.
type ErrNoIPAddr struct{}

func (enip *ErrNoIPAddr) Error() string {
	return "bridge has no IPv4 address configured"
}

// InternalError denotes the type of this error
func (enip *ErrNoIPAddr) InternalError() {}

// ErrInvalidGateway is returned when the user provided default gateway (v4/v6) is not valid.
type ErrInvalidGateway struct{}

func (eig *ErrInvalidGateway) Error() string {
	return "default gateway ip must be part of the network"
}

// InvalidParameter denotes the type of this error
func (eig *ErrInvalidGateway) InvalidParameter() {}

// ErrInvalidMtu is returned when the user provided MTU is not valid.
type ErrInvalidMtu int

func (eim ErrInvalidMtu) Error() string {
	return fmt.Sprintf("invalid MTU number: %d", int(eim))
}

// InvalidParameter denotes the type of this error
func (eim ErrInvalidMtu) InvalidParameter() {}

// InvalidNetworkIDError is returned when the passed
// network id for an existing network is not a known id.
type InvalidNetworkIDError string

func (inie InvalidNetworkIDError) Error() string {
	return fmt.Sprintf("invalid network id %s", string(inie))
}

// NotFound denotes the type of this error
func (inie InvalidNetworkIDError) NotFound() {}

// InvalidEndpointIDError is returned when the passed
// endpoint id is not valid.
type InvalidEndpointIDError string

func (ieie InvalidEndpointIDError) Error() string {
	return fmt.Sprintf("invalid endpoint id: %s", string(ieie))
}

// InvalidParameter denotes the type of this error
func (ieie InvalidEndpointIDError) InvalidParameter() {}

// EndpointNotFoundError is returned when the no endpoint
// with the passed endpoint id is found.
type EndpointNotFoundError string

func (enfe EndpointNotFoundError) Error() string {
	return fmt.Sprintf("endpoint not found: %s", string(enfe))
}

// NotFound denotes the type of this error
func (enfe EndpointNotFoundError) NotFound() {}

// NonDefaultBridgeExistError is returned when a non-default
// bridge config is passed but it does not already exist.
type NonDefaultBridgeExistError string

func (ndbee NonDefaultBridgeExistError) Error() string {
	return fmt.Sprintf("bridge device with non default name %s must be created manually", string(ndbee))
}

// Forbidden denotes the type of this error
func (ndbee NonDefaultBridgeExistError) Forbidden() {}

// IPv4AddrAddError is returned when IPv4 address could not be added to the bridge.
type IPv4AddrAddError struct {
	IP  *net.IPNet
	Err error
}

func (ipv4 *IPv4AddrAddError) Error() string {
	return fmt.Sprintf("failed to add IPv4 address %s to bridge: %v", ipv4.IP, ipv4.Err)
}

// InternalError denotes the type of this error
func (ipv4 *IPv4AddrAddError) InternalError() {}

// IPv4AddrNoMatchError is returned when the bridge's IPv4 address does not match configured.
type IPv4AddrNoMatchError struct {
	IP    net.IP
	CfgIP net.IP
}

func (ipv4 *IPv4AddrNoMatchError) Error() string {
	return fmt.Sprintf("bridge IPv4 (%s) does not match requested configuration %s", ipv4.IP, ipv4.CfgIP)
}
