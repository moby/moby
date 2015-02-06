package nat

// nat is a convenience package for docker's manipulation of strings describing
// network ports.

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/parsers"
)

const (
	PortSpecTemplate       = "ip:hostPort:containerPort"
	PortSpecTemplateFormat = "ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort | containerPort"
)

type PortBinding struct {
	HostIp   string
	HostPort string
}

type PortMap map[Port][]PortBinding

type PortSet map[Port]struct{}

// 80/tcp
type Port string

func NewPort(proto, port string) Port {
	return Port(fmt.Sprintf("%s/%s", port, proto))
}

func ParsePort(rawPort string) (int, error) {
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return 0, err
	}
	return int(port), nil
}

func (p Port) Proto() string {
	proto, _ := SplitProtoPort(string(p))
	return proto
}

func (p Port) Port() string {
	_, port := SplitProtoPort(string(p))
	return port
}

func (p Port) Int() int {
	port, err := ParsePort(p.Port())
	if err != nil {
		panic(err)
	}
	return port
}

// Splits a port in the format of proto/port
func SplitProtoPort(rawPort string) (string, string) {
	parts := strings.Split(rawPort, "/")
	l := len(parts)
	if len(rawPort) == 0 || l == 0 || len(parts[0]) == 0 {
		return "", ""
	}
	if l == 1 {
		return "tcp", rawPort
	}
	if len(parts[1]) == 0 {
		return "tcp", parts[0]
	}
	return parts[1], parts[0]
}

func validateProto(proto string) bool {
	for _, availableProto := range []string{"tcp", "udp"} {
		if availableProto == proto {
			return true
		}
	}
	return false
}

// We will receive port specs in the format of ip:public:private/proto and these need to be
// parsed in the internal types
func ParsePortSpecs(ports []string) (map[Port]struct{}, map[Port][]PortBinding, error) {
	var (
		exposedPorts = make(map[Port]struct{}, len(ports))
		bindings     = make(map[Port][]PortBinding)
	)

	for _, rawPort := range ports {
		proto := "tcp"

		if i := strings.LastIndex(rawPort, "/"); i != -1 {
			proto = rawPort[i+1:]
			rawPort = rawPort[:i]
		}
		if !strings.Contains(rawPort, ":") {
			rawPort = fmt.Sprintf("::%s", rawPort)
		} else if len(strings.Split(rawPort, ":")) == 2 {
			rawPort = fmt.Sprintf(":%s", rawPort)
		}

		parts, err := parsers.PartParser(PortSpecTemplate, rawPort)
		if err != nil {
			return nil, nil, err
		}

		var (
			containerPort = parts["containerPort"]
			rawIp         = parts["ip"]
			hostPort      = parts["hostPort"]
		)

		if rawIp != "" && net.ParseIP(rawIp) == nil {
			return nil, nil, fmt.Errorf("Invalid ip address: %s", rawIp)
		}
		if containerPort == "" {
			return nil, nil, fmt.Errorf("No port specified: %s<empty>", rawPort)
		}

		startPort, endPort, err := parsers.ParsePortRange(containerPort)
		if err != nil {
			return nil, nil, fmt.Errorf("Invalid containerPort: %s", containerPort)
		}

		var startHostPort, endHostPort uint64 = 0, 0
		if len(hostPort) > 0 {
			startHostPort, endHostPort, err = parsers.ParsePortRange(hostPort)
			if err != nil {
				return nil, nil, fmt.Errorf("Invalid hostPort: %s", hostPort)
			}
		}

		if hostPort != "" && (endPort-startPort) != (endHostPort-startHostPort) {
			return nil, nil, fmt.Errorf("Invalid ranges specified for container and host Ports: %s and %s", containerPort, hostPort)
		}

		if !validateProto(strings.ToLower(proto)) {
			return nil, nil, fmt.Errorf("Invalid proto: %s", proto)
		}

		for i := uint64(0); i <= (endPort - startPort); i++ {
			containerPort = strconv.FormatUint(startPort+i, 10)
			if len(hostPort) > 0 {
				hostPort = strconv.FormatUint(startHostPort+i, 10)
			}
			port := NewPort(strings.ToLower(proto), containerPort)
			if _, exists := exposedPorts[port]; !exists {
				exposedPorts[port] = struct{}{}
			}

			binding := PortBinding{
				HostIp:   rawIp,
				HostPort: hostPort,
			}
			bslice, exists := bindings[port]
			if !exists {
				bslice = []PortBinding{}
			}
			bindings[port] = append(bslice, binding)
		}
	}
	return exposedPorts, bindings, nil
}
