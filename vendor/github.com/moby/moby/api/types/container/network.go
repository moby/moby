package container

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type PortProto string

const (
	TCP  PortProto = "tcp"
	UDP  PortProto = "udp"
	SCTP PortProto = "sctp"
)

// Port is a type representing a single port number and protocol in the format "80/tcp".
type Port struct {
	num   uint16
	proto PortProto
}

// ParsePort parses s as a [Port].
func ParsePort(s string) (Port, error) {
	if s == "" {
		return Port{}, errors.New("value is empty")
	}

	port, proto, ok := strings.Cut(s, "/")
	if !ok {
		proto = "tcp"
	}

	portVal, err := parsePortNumber(port)
	if err != nil {
		return Port{}, fmt.Errorf("invalid port '%s': %w", port, err)
	}

	return Port{num: portVal, proto: PortProto(proto)}, nil
}

// MustParsePort calls [ParsePort](s) and panics on error.
func MustParsePort(s string) Port {
	p, err := ParsePort(s)
	if err != nil {
		panic(err)
	}
	return p
}

func (p Port) Num() uint16 {
	return p.num
}

func (p Port) Proto() PortProto {
	return p.proto
}

func (p Port) String() string {
	return fmt.Sprintf("%d/%s", p.num, p.proto)
}

func (p Port) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *Port) UnmarshalText(text []byte) error {
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

type PortRange struct {
	start uint16
	end   uint16
	proto PortProto
}

func ParsePortRange(s string) (PortRange, error) {
	if s == "" {
		return PortRange{}, errors.New("value is empty")
	}

	portRange, proto, ok := strings.Cut(s, "/")
	if !ok {
		proto = "tcp"
	}

	start, end, ok := strings.Cut(portRange, "-")
	startVal, err := parsePortNumber(start)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid start port '%s': %w", start, err)
	}

	if !ok || start == end {
		return PortRange{start: startVal, end: startVal, proto: PortProto(proto)}, nil
	}

	endVal, err := parsePortNumber(end)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid end port '%s': %w", end, err)
	}
	if endVal < startVal {
		return PortRange{}, errors.New("invalid port range: " + s)
	}
	return PortRange{start: startVal, end: endVal, proto: PortProto(proto)}, nil
}

// MustParsePortRange calls [ParsePortRange](s) and panics on error.
func MustParsePortRange(s string) PortRange {
	pr, err := ParsePortRange(s)
	if err != nil {
		panic(err)
	}
	return pr
}

func (pr PortRange) Start() uint16 {
	return pr.start
}

func (pr PortRange) End() uint16 {
	return pr.end
}

func (pr PortRange) Proto() PortProto {
	return pr.proto
}

func (pr PortRange) String() string {
	return fmt.Sprintf("%d-%d/%s", pr.start, pr.end, pr.proto)
}

func (pr PortRange) MarshalText() ([]byte, error) {
	return []byte(pr.String()), nil
}

func (pr *PortRange) UnmarshalText(text []byte) error {
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
