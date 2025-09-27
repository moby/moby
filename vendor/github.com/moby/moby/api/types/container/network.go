package container

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unique"
)

type PortProto string

const (
	TCP  PortProto = "tcp"
	UDP  PortProto = "udp"
	SCTP PortProto = "sctp"
)

// Sentinel port proto value for zero Port and PortRange values.
var protoZero unique.Handle[PortProto]

// Port is a type representing a single port number and protocol in the format "<portnum>/[<proto>]".
//
// The zero port value, i.e. Port{}, is invalid; use [ParsePort] to create a valid Port value.
type Port struct {
	num   uint16
	proto unique.Handle[PortProto]
}

// ParsePort parses s as a [Port].
//
// It normalizes the provided protocol such that "80/tcp", "80/TCP", and "80/tCp" are equivalent.
// If a port number is provided, but no protocol, the default ("tcp") protocol is returned.
func ParsePort(s string) (Port, error) {
	if s == "" {
		return Port{}, errors.New("value is empty")
	}

	port, proto, _ := strings.Cut(s, "/")

	portNum, err := parsePortNumber(port)
	if err != nil {
		return Port{}, fmt.Errorf("invalid port '%s': %w", port, err)
	}

	portProto := normalizePortProto(proto)

	return Port{num: portNum, proto: unique.Make(portProto)}, nil
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

// PortFrom returns a Port with the given number and protocol.
//
// If no protocol is specified (i.e. proto == ""), then PortFrom returns Port{}.
func PortFrom(num uint16, proto PortProto) Port {
	if proto == "" {
		return Port{}
	}
	normalized := normalizePortProto(string(proto))
	return Port{num: num, proto: unique.Make(PortProto(normalized))}
}

func (p Port) Num() uint16 {
	return p.num
}

func (p Port) Proto() PortProto {
	return p.proto.Value()
}

// IsZero reports whether the [Port] is the zero value.
func (p Port) IsZero() bool {
	return p.proto == protoZero
}

// IsValid reports whether the [Port] is an initialized valid port (not the zero value).
func (p Port) IsValid() bool {
	return p.proto != protoZero
}

func (p Port) String() string {
	switch p.proto {
	case protoZero:
		return "invalid port"
	default:
		return string(p.appendText(nil))
	}
}

func (p Port) AppendText(b []byte) ([]byte, error) {
	return p.appendText(b), nil
}

func (p Port) appendText(b []byte) []byte {
	if p.IsZero() {
		return b
	}
	return fmt.Appendf(b, "%d/%s", p.num, p.proto.Value())
}

func (p Port) MarshalText() ([]byte, error) {
	return p.AppendText(nil)
}

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

func (p Port) Range() PortRange {
	return PortRange{start: p.num, end: p.num, proto: p.proto}
}

// PortSet is a collection of structs indexed by [Port].
type PortSet = map[Port]struct{}

// PortBinding represents a binding between a Host IP address and a Host Port.
type PortBinding struct {
	// HostIP is the host IP Address
	HostIP string `json:"HostIp"`
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
	proto unique.Handle[PortProto]
}

// ParsePortRange parses s as a [PortRange].
//
// It normalizes the provided protocol such that "80-90/tcp", "80-90/TCP", and "80-90/tCp" are equivalent.
// If a port number range is provided, but no protocol, the default ("tcp") protocol is returned.
func ParsePortRange(s string) (PortRange, error) {
	if s == "" {
		return PortRange{}, errors.New("value is empty")
	}

	portRange, proto, _ := strings.Cut(s, "/")

	start, end, ok := strings.Cut(portRange, "-")
	startVal, err := parsePortNumber(start)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid start port '%s': %w", start, err)
	}

	portProto := normalizePortProto(proto)

	if !ok || start == end {
		return PortRange{start: startVal, end: startVal, proto: unique.Make(portProto)}, nil
	}

	endVal, err := parsePortNumber(end)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid end port '%s': %w", end, err)
	}
	if endVal < startVal {
		return PortRange{}, errors.New("invalid port range: " + s)
	}
	return PortRange{start: startVal, end: endVal, proto: unique.Make(portProto)}, nil
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
// If end < start, then PortRangeFrom returns an error.
func PortRangeFrom(start, end uint16, proto PortProto) (PortRange, error) {
	if end < start {
		return PortRange{}, errors.New("invalid port range")
	}
	if proto == "" {
		return PortRange{}, nil
	}
	normalized := normalizePortProto(string(proto))
	return PortRange{start: start, end: end, proto: unique.Make(normalized)}, nil
}

func (pr PortRange) Start() uint16 {
	return pr.start
}

func (pr PortRange) End() uint16 {
	return pr.end
}

func (pr PortRange) Proto() PortProto {
	return pr.proto.Value()
}

// IsZero reports whether the [PortRange] is the zero value.
func (pr PortRange) IsZero() bool {
	return pr.proto == protoZero
}

// IsValid reports whether the [PortRange] is an initialized valid port range (not the zero value).
func (pr PortRange) IsValid() bool {
	return pr.proto != protoZero
}

func (pr PortRange) String() string {
	switch pr.proto {
	case protoZero:
		return "invalid port range"
	default:
		return string(pr.appendText(nil))
	}
}

func (pr PortRange) AppendText(b []byte) ([]byte, error) {
	return pr.appendText(b), nil
}

func (pr PortRange) appendText(b []byte) []byte {
	if pr.IsZero() {
		return b
	}
	return fmt.Appendf(b, "%d-%d/%s", pr.start, pr.end, pr.proto.Value())
}

func (pr PortRange) MarshalText() ([]byte, error) {
	return pr.AppendText(nil)
}

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

func (pr PortRange) Range() PortRange {
	return pr
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

// normalizePortProto normalizes the protocol string, returning an error if it contains invalid characters.
func normalizePortProto(proto string) PortProto {
	// If protocol is not specified, default to "tcp".
	if proto == "" {
		return TCP
	}

	// Normalize protocol to lowercase.
	// i.e. "tcp" == "TCP" == "tCp"
	proto = strings.ToLower(proto)

	return PortProto(proto)
}
