package container

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// PortSet is a collection of structs indexed by [PortProto].
type PortSet = map[PortProto]struct{}

// PortBinding represents a binding between a Host IP address and a Host Port.
type PortBinding struct {
	// HostIP is the host IP Address
	HostIP string `json:"HostIp"`
	// HostPort is the host port number
	HostPort string
}

// PortMap is a collection of [PortBinding] indexed by [PortProto].
type PortMap = map[PortProto][]PortBinding

// PortProto is a string containing port number and protocol in the format "80/tcp".
// It is the same as [PortRangeProto], but used in places where we only expect
// a single port to be used (not a range).
type PortProto string

// Proto returns the protocol of a Port
func (p PortProto) Proto() string {
	_, proto, _ := strings.Cut(string(p), "/")
	if proto == "" {
		proto = "tcp"
	}
	return proto
}

// Port returns the port number of a Port
func (p PortProto) Port() string {
	port, _, _ := strings.Cut(string(p), "/")
	return port
}

// Int returns the port number of a Port as an int.
func (p PortProto) Int() (int, error) {
	port, _, _ := strings.Cut(string(p), "/")
	return parsePortNumber(port)
}

// PortRangeProto is a string containing a range of port numbers and protocol in
// the format "80-90/tcp". It the same as [PortProto], but used in places where
// we expect a port-range to be used.
type PortRangeProto string

func (pr PortRangeProto) PortRange() string {
	portRange, _, _ := strings.Cut(string(pr), "/")
	return portRange
}

func (pr PortRangeProto) Proto() string {
	_, proto, _ := strings.Cut(string(pr), "/")
	if proto == "" {
		proto = "tcp"
	}
	return proto
}

// Range returns the start/end port numbers of a Port range as ints
func (pr PortRangeProto) Range() (int, int, error) {
	portRange, _, _ := strings.Cut(string(pr), "/")
	if portRange == "" {
		return 0, 0, nil
	}
	start, end, ok := strings.Cut(portRange, "-")
	startPort, err := parsePortNumber(start)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start port '%s': %w", start, err)
	}
	if !ok || start == end {
		return startPort, startPort, nil
	}

	endPort, err := parsePortNumber(end)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end port '%s': %w", end, err)
	}
	if endPort < startPort {
		return 0, 0, errors.New("invalid port range: " + portRange)
	}
	return startPort, endPort, nil
}

// parsePortNumber parses rawPort into an int, unwrapping strconv errors
// and returning a single "out of range" error for any value outside 0–65535.
func parsePortNumber(rawPort string) (int, error) {
	if rawPort == "" {
		return 0, errors.New("value is empty")
	}
	port, err := strconv.ParseInt(rawPort, 10, 0)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) {
			err = numErr.Err
		}
		return 0, err
	}
	if port < 0 || port > 65535 {
		return 0, errors.New("value out of range (0–65535)")
	}

	return int(port), nil
}
