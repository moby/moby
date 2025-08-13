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

// Int returns the port number of a Port as an int. It assumes [Port]
// is valid, and returns 0 otherwise.
func (p PortProto) Int() int {
	// We don't need to check for an error because we're going to
	// assume that any error would have been found, and reported, in [NewPort]
	port, _ := parsePortNumber(p.Port())
	return port
}

// Range returns the start/end port numbers of a Port range as ints
func (p PortProto) Range() (int, int, error) {
	portRange := p.Port()
	if portRange == "" {
		return 0, 0, nil
	}
	return parsePortRange(portRange)
}

// parsePortRange parses and validates the specified string as a port range (e.g., "8000-9000").
func parsePortRange(ports string) (startPort, endPort int, _ error) {
	if ports == "" {
		return 0, 0, errors.New("empty string specified for ports")
	}
	start, end, ok := strings.Cut(ports, "-")

	startPort, err := parsePortNumber(start)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start port '%s': %w", start, err)
	}
	if !ok || start == end {
		return startPort, startPort, nil
	}

	endPort, err = parsePortNumber(end)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end port '%s': %w", end, err)
	}
	if endPort < startPort {
		return 0, 0, errors.New("invalid port range: " + ports)
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
