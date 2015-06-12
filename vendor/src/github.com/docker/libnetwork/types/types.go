// Package types contains types that are common across libnetwork project
package types

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

// UUID represents a globally unique ID of various resources like network and endpoint
type UUID string

// TransportPort represent a local Layer 4 endpoint
type TransportPort struct {
	Proto Protocol
	Port  uint16
}

// GetCopy returns a copy of this TransportPort structure instance
func (t *TransportPort) GetCopy() TransportPort {
	return TransportPort{Proto: t.Proto, Port: t.Port}
}

// PortBinding represent a port binding between the container and the host
type PortBinding struct {
	Proto    Protocol
	IP       net.IP
	Port     uint16
	HostIP   net.IP
	HostPort uint16
}

// HostAddr returns the host side transport address
func (p PortBinding) HostAddr() (net.Addr, error) {
	switch p.Proto {
	case UDP:
		return &net.UDPAddr{IP: p.HostIP, Port: int(p.HostPort)}, nil
	case TCP:
		return &net.TCPAddr{IP: p.HostIP, Port: int(p.HostPort)}, nil
	default:
		return nil, ErrInvalidProtocolBinding(p.Proto.String())
	}
}

// ContainerAddr returns the container side transport address
func (p PortBinding) ContainerAddr() (net.Addr, error) {
	switch p.Proto {
	case UDP:
		return &net.UDPAddr{IP: p.IP, Port: int(p.Port)}, nil
	case TCP:
		return &net.TCPAddr{IP: p.IP, Port: int(p.Port)}, nil
	default:
		return nil, ErrInvalidProtocolBinding(p.Proto.String())
	}
}

// GetCopy returns a copy of this PortBinding structure instance
func (p *PortBinding) GetCopy() PortBinding {
	return PortBinding{
		Proto:    p.Proto,
		IP:       GetIPCopy(p.IP),
		Port:     p.Port,
		HostIP:   GetIPCopy(p.HostIP),
		HostPort: p.HostPort,
	}
}

// Equal checks if this instance of PortBinding is equal to the passed one
func (p *PortBinding) Equal(o *PortBinding) bool {
	if p == o {
		return true
	}

	if o == nil {
		return false
	}

	if p.Proto != o.Proto || p.Port != o.Port || p.HostPort != o.HostPort {
		return false
	}

	if p.IP != nil {
		if !p.IP.Equal(o.IP) {
			return false
		}
	} else {
		if o.IP != nil {
			return false
		}
	}

	if p.HostIP != nil {
		if !p.HostIP.Equal(o.HostIP) {
			return false
		}
	} else {
		if o.HostIP != nil {
			return false
		}
	}

	return true
}

// ErrInvalidProtocolBinding is returned when the port binding protocol is not valid.
type ErrInvalidProtocolBinding string

func (ipb ErrInvalidProtocolBinding) Error() string {
	return fmt.Sprintf("invalid transport protocol: %s", string(ipb))
}

const (
	// ICMP is for the ICMP ip protocol
	ICMP = 1
	// TCP is for the TCP ip protocol
	TCP = 6
	// UDP is for the UDP ip protocol
	UDP = 17
)

// Protocol represents a IP protocol number
type Protocol uint8

func (p Protocol) String() string {
	switch p {
	case ICMP:
		return "icmp"
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	default:
		return fmt.Sprintf("%d", p)
	}
}

// ParseProtocol returns the respective Protocol type for the passed string
func ParseProtocol(s string) Protocol {
	switch strings.ToLower(s) {
	case "icmp":
		return ICMP
	case "udp":
		return UDP
	case "tcp":
		return TCP
	default:
		return 0
	}
}

// GetMacCopy returns a copy of the passed MAC address
func GetMacCopy(from net.HardwareAddr) net.HardwareAddr {
	to := make(net.HardwareAddr, len(from))
	copy(to, from)
	return to
}

// GetIPCopy returns a copy of the passed IP address
func GetIPCopy(from net.IP) net.IP {
	to := make(net.IP, len(from))
	copy(to, from)
	return to
}

// GetIPNetCopy returns a copy of the passed IP Network
func GetIPNetCopy(from *net.IPNet) *net.IPNet {
	if from == nil {
		return nil
	}
	bm := make(net.IPMask, len(from.Mask))
	copy(bm, from.Mask)
	return &net.IPNet{IP: GetIPCopy(from.IP), Mask: bm}
}

// CompareIPNet returns equal if the two IP Networks are equal
func CompareIPNet(a, b *net.IPNet) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.IP.Equal(b.IP) && bytes.Equal(a.Mask, b.Mask)
}

const (
	// NEXTHOP indicates a StaticRoute with an IP next hop.
	NEXTHOP = iota

	// CONNECTED indicates a StaticRoute with a interface for directly connected peers.
	CONNECTED
)

// StaticRoute is a statically-provisioned IP route.
type StaticRoute struct {
	Destination *net.IPNet

	RouteType int // NEXT_HOP or CONNECTED

	// NextHop will be resolved by the kernel (i.e. as a loose hop).
	NextHop net.IP

	// InterfaceID must refer to a defined interface on the
	// Endpoint to which the routes are specified.  Routes specified this way
	// are interpreted as directly connected to the specified interface (no
	// next hop will be used).
	InterfaceID int
}

// GetCopy returns a copy of this StaticRoute structure
func (r *StaticRoute) GetCopy() *StaticRoute {
	d := GetIPNetCopy(r.Destination)
	nh := GetIPCopy(r.NextHop)
	return &StaticRoute{Destination: d,
		RouteType:   r.RouteType,
		NextHop:     nh,
		InterfaceID: r.InterfaceID}
}

/******************************
 * Well-known Error Interfaces
 ******************************/

// MaskableError is an interface for errors which can be ignored by caller
type MaskableError interface {
	// Maskable makes implementer into MaskableError type
	Maskable()
}

// BadRequestError is an interface for errors originated by a bad request
type BadRequestError interface {
	// BadRequest makes implementer into BadRequestError type
	BadRequest()
}

// NotFoundError is an interface for errors raised because a needed resource is not available
type NotFoundError interface {
	// NotFound makes implementer into NotFoundError type
	NotFound()
}

// ForbiddenError is an interface for errors which denote an valid request that cannot be honored
type ForbiddenError interface {
	// Forbidden makes implementer into ForbiddenError type
	Forbidden()
}

// NoServiceError  is an interface for errors returned when the required service is not available
type NoServiceError interface {
	// NoService makes implementer into NoServiceError type
	NoService()
}

// TimeoutError  is an interface for errors raised because of timeout
type TimeoutError interface {
	// Timeout makes implementer into TimeoutError type
	Timeout()
}

// NotImplementedError  is an interface for errors raised because of requested functionality is not yet implemented
type NotImplementedError interface {
	// NotImplemented makes implementer into NotImplementedError type
	NotImplemented()
}

// InternalError is an interface for errors raised because of an internal error
type InternalError interface {
	// Internal makes implementer into InternalError type
	Internal()
}

/******************************
 * Weel-known Error Formatters
 ******************************/

// BadRequestErrorf creates an instance of BadRequestError
func BadRequestErrorf(format string, params ...interface{}) error {
	return badRequest(fmt.Sprintf(format, params...))
}

// NotFoundErrorf creates an instance of NotFoundError
func NotFoundErrorf(format string, params ...interface{}) error {
	return notFound(fmt.Sprintf(format, params...))
}

// ForbiddenErrorf creates an instance of ForbiddenError
func ForbiddenErrorf(format string, params ...interface{}) error {
	return forbidden(fmt.Sprintf(format, params...))
}

// NoServiceErrorf creates an instance of NoServiceError
func NoServiceErrorf(format string, params ...interface{}) error {
	return noService(fmt.Sprintf(format, params...))
}

// NotImplementedErrorf creates an instance of NotImplementedError
func NotImplementedErrorf(format string, params ...interface{}) error {
	return notImpl(fmt.Sprintf(format, params...))
}

// TimeoutErrorf creates an instance of TimeoutError
func TimeoutErrorf(format string, params ...interface{}) error {
	return timeout(fmt.Sprintf(format, params...))
}

// InternalErrorf creates an instance of InternalError
func InternalErrorf(format string, params ...interface{}) error {
	return internal(fmt.Sprintf(format, params...))
}

// InternalMaskableErrorf creates an instance of InternalError and MaskableError
func InternalMaskableErrorf(format string, params ...interface{}) error {
	return maskInternal(fmt.Sprintf(format, params...))
}

/***********************
 * Internal Error Types
 ***********************/
type badRequest string

func (br badRequest) Error() string {
	return string(br)
}
func (br badRequest) BadRequest() {}

type maskBadRequest string

type notFound string

func (nf notFound) Error() string {
	return string(nf)
}
func (nf notFound) NotFound() {}

type forbidden string

func (frb forbidden) Error() string {
	return string(frb)
}
func (frb forbidden) Forbidden() {}

type noService string

func (ns noService) Error() string {
	return string(ns)
}
func (ns noService) NoService() {}

type maskNoService string

type timeout string

func (to timeout) Error() string {
	return string(to)
}
func (to timeout) Timeout() {}

type notImpl string

func (ni notImpl) Error() string {
	return string(ni)
}
func (ni notImpl) NotImplemented() {}

type internal string

func (nt internal) Error() string {
	return string(nt)
}
func (nt internal) Internal() {}

type maskInternal string

func (mnt maskInternal) Error() string {
	return string(mnt)
}
func (mnt maskInternal) Internal() {}
func (mnt maskInternal) Maskable() {}
