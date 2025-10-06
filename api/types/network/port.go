package network

import (
	"errors"
	"fmt"
	"iter"
	"net/netip"
	"strconv"
	"strings"
	"unique"
)

// IPProtocol represents a network protocol for a port.
type IPProtocol string

const (
	TCP  IPProtocol = "tcp"
	UDP  IPProtocol = "udp"
	SCTP IPProtocol = "sctp"
)

// Sentinel port proto value for zero Port and PortRange values.
var protoZero unique.Handle[IPProtocol]

// Port is a type representing a single port number and protocol in the format "<portnum>/[<proto>]".
//
// The zero port value, i.e. Port{}, is invalid; use [ParsePort] to create a valid Port value.
type Port struct {
	num   uint16
	proto unique.Handle[IPProtocol]
}

// ParsePort parses s as a [Port].
//
// It normalizes the provided protocol such that "80/tcp", "80/TCP", and "80/tCp" are equivalent.
// If a port number is provided, but no protocol, the default ("tcp") protocol is returned.
func ParsePort(s string) (Port, error) {
	if s == "" {
		return Port{}, errors.New("invalid port: value is empty")
	}

	port, proto, _ := strings.Cut(s, "/")

	portNum, err := parsePortNumber(port)
	if err != nil {
		return Port{}, fmt.Errorf("invalid port '%s': %w", port, err)
	}

	normalizedPortProto := normalizePortProto(proto)
	return Port{num: portNum, proto: normalizedPortProto}, nil
}

// MustParsePort calls [ParsePort](s) and panics on error.
//
// It is intended for use in tests with hard-coded strings.
func MustParsePort(s string) Port {
	p, err := ParsePort(s)
	if err != nil {
		panic(err)
	}
	return p
}

// PortFrom returns a [Port] with the given number and protocol.
//
// If no protocol is specified (i.e. proto == ""), then PortFrom returns Port{}, false.
func PortFrom(num uint16, proto IPProtocol) (p Port, ok bool) {
	if proto == "" {
		return Port{}, false
	}
	normalized := normalizePortProto(string(proto))
	return Port{num: num, proto: normalized}, true
}

// Num returns p's port number.
func (p Port) Num() uint16 {
	return p.num
}

// Proto returns p's network protocol.
func (p Port) Proto() IPProtocol {
	return p.proto.Value()
}

// IsZero reports whether p is the zero value.
func (p Port) IsZero() bool {
	return p.proto == protoZero
}

// IsValid reports whether p is an initialized valid port (not the zero value).
func (p Port) IsValid() bool {
	return p.proto != protoZero
}

// String returns a string representation of the port in the format "<portnum>/<proto>".
// If the port is the zero value, it returns "invalid port".
func (p Port) String() string {
	switch p.proto {
	case protoZero:
		return "invalid port"
	default:
		return string(p.AppendTo(nil))
	}
}

// AppendText implements [encoding.TextAppender] interface.
// It is the same as [Port.AppendTo] but returns an error to satisfy the interface.
func (p Port) AppendText(b []byte) ([]byte, error) {
	return p.AppendTo(b), nil
}

// AppendTo appends a text encoding of p to b and returns the extended buffer.
func (p Port) AppendTo(b []byte) []byte {
	if p.IsZero() {
		return b
	}
	return fmt.Appendf(b, "%d/%s", p.num, p.proto.Value())
}

// MarshalText implements [encoding.TextMarshaler] interface.
func (p Port) MarshalText() ([]byte, error) {
	return p.AppendText(nil)
}

// UnmarshalText implements [encoding.TextUnmarshaler] interface.
func (p *Port) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*p = Port{}
		return nil
	}

	port, err := ParsePort(string(text))
	if err != nil {
		return err
	}

	*p = port
	return nil
}

// Range returns a [PortRange] representing the single port.
func (p Port) Range() PortRange {
	return PortRange{start: p.num, end: p.num, proto: p.proto}
}

// PortSet is a collection of structs indexed by [Port].
type PortSet = map[Port]struct{}

// PortBinding represents a binding between a Host IP address and a Host Port.
type PortBinding struct {
	// HostIP is the host IP Address
	HostIP netip.Addr `json:"HostIp"`
	// HostPort is the host port number
	HostPort string `json:"HostPort"`
}

// PortMap is a collection of [PortBinding] indexed by [Port].
type PortMap = map[Port][]PortBinding

// PortRange represents a range of port numbers and a protocol in the format "8000-9000/tcp".
//
// The zero port range value, i.e. PortRange{}, is invalid; use [ParsePortRange] to create a valid PortRange value.
type PortRange struct {
	start uint16
	end   uint16
	proto unique.Handle[IPProtocol]
}

// ParsePortRange parses s as a [PortRange].
//
// It normalizes the provided protocol such that "80-90/tcp", "80-90/TCP", and "80-90/tCp" are equivalent.
// If a port number range is provided, but no protocol, the default ("tcp") protocol is returned.
func ParsePortRange(s string) (PortRange, error) {
	if s == "" {
		return PortRange{}, errors.New("invalid port range: value is empty")
	}

	portRange, proto, _ := strings.Cut(s, "/")

	start, end, ok := strings.Cut(portRange, "-")
	startVal, err := parsePortNumber(start)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid start port '%s': %w", start, err)
	}

	portProto := normalizePortProto(proto)

	if !ok || start == end {
		return PortRange{start: startVal, end: startVal, proto: portProto}, nil
	}

	endVal, err := parsePortNumber(end)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid end port '%s': %w", end, err)
	}
	if endVal < startVal {
		return PortRange{}, errors.New("invalid port range: " + s)
	}
	return PortRange{start: startVal, end: endVal, proto: portProto}, nil
}

// MustParsePortRange calls [ParsePortRange](s) and panics on error.
// It is intended for use in tests with hard-coded strings.
func MustParsePortRange(s string) PortRange {
	pr, err := ParsePortRange(s)
	if err != nil {
		panic(err)
	}
	return pr
}

// PortRangeFrom returns a [PortRange] with the given start and end port numbers and protocol.
//
// If end < start or no protocol is specified (i.e. proto == ""), then PortRangeFrom returns PortRange{}, false.
func PortRangeFrom(start, end uint16, proto IPProtocol) (pr PortRange, ok bool) {
	if end < start || proto == "" {
		return PortRange{}, false
	}
	normalized := normalizePortProto(string(proto))
	return PortRange{start: start, end: end, proto: normalized}, true
}

// Start returns pr's start port number.
func (pr PortRange) Start() uint16 {
	return pr.start
}

// End returns pr's end port number.
func (pr PortRange) End() uint16 {
	return pr.end
}

// Proto returns pr's network protocol.
func (pr PortRange) Proto() IPProtocol {
	return pr.proto.Value()
}

// IsZero reports whether pr is the zero value.
func (pr PortRange) IsZero() bool {
	return pr.proto == protoZero
}

// IsValid reports whether pr is an initialized valid port range (not the zero value).
func (pr PortRange) IsValid() bool {
	return pr.proto != protoZero
}

// String returns a string representation of the port range in the format "<start>-<end>/<proto>" or "<portnum>/<proto>" if start == end.
// If the port range is the zero value, it returns "invalid port range".
func (pr PortRange) String() string {
	switch pr.proto {
	case protoZero:
		return "invalid port range"
	default:
		return string(pr.AppendTo(nil))
	}
}

// AppendText implements [encoding.TextAppender] interface.
// It is the same as [PortRange.AppendTo] but returns an error to satisfy the interface.
func (pr PortRange) AppendText(b []byte) ([]byte, error) {
	return pr.AppendTo(b), nil
}

// AppendTo appends a text encoding of pr to b and returns the extended buffer.
func (pr PortRange) AppendTo(b []byte) []byte {
	if pr.IsZero() {
		return b
	}
	if pr.start == pr.end {
		return fmt.Appendf(b, "%d/%s", pr.start, pr.proto.Value())
	}
	return fmt.Appendf(b, "%d-%d/%s", pr.start, pr.end, pr.proto.Value())
}

// MarshalText implements [encoding.TextMarshaler] interface.
func (pr PortRange) MarshalText() ([]byte, error) {
	return pr.AppendText(nil)
}

// UnmarshalText implements [encoding.TextUnmarshaler] interface.
func (pr *PortRange) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*pr = PortRange{}
		return nil
	}

	portRange, err := ParsePortRange(string(text))
	if err != nil {
		return err
	}
	*pr = portRange
	return nil
}

// Range returns pr.
func (pr PortRange) Range() PortRange {
	return pr
}

// All returns an iterator over all the individual ports in the range.
//
// For example:
//
//	for port := range pr.All() {
//	    // ...
//	}
func (pr PortRange) All() iter.Seq[Port] {
	return func(yield func(Port) bool) {
		for i := uint32(pr.Start()); i <= uint32(pr.End()); i++ {
			if !yield(Port{num: uint16(i), proto: pr.proto}) {
				return
			}
		}
	}
}

// parsePortNumber parses rawPort into an int, unwrapping strconv errors
// and returning a single "out of range" error for any value outside 0–65535.
func parsePortNumber(rawPort string) (uint16, error) {
	if rawPort == "" {
		return 0, errors.New("value is empty")
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) {
			err = numErr.Err
		}
		return 0, err
	}

	return uint16(port), nil
}

// normalizePortProto normalizes the protocol string such that "tcp", "TCP", and "tCp" are equivalent.
// If proto is not specified, it defaults to "tcp".
func normalizePortProto(proto string) unique.Handle[IPProtocol] {
	if proto == "" {
		return unique.Make(TCP)
	}

	proto = strings.ToLower(proto)

	return unique.Make(IPProtocol(proto))
}
