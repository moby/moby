package opts

import (
	"encoding/csv"
	"fmt"
	"github.com/docker/docker/api/types/swarm"
	"strings"
)

const (
	networkOptName  = "name"
	networkOptAlias = "alias"
	networkOptIP    = "ip"
	networkOptIPv6  = "ipv6"
)

// NetworkOpt represents a network config in swarm mode.
type NetworkOpt struct {
	networkAttachments []swarm.NetworkAttachmentConfig
}

// Set networkopts value
func (n *NetworkOpt) Set(value string) error {
	csvReader := csv.NewReader(strings.NewReader(value))
	fields, err := csvReader.Read()
	if err != nil {
		return err
	}
	var netAttach swarm.NetworkAttachmentConfig
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		//Support legacy non csv format
		if len(parts) == 1 {
			netAttach.Target = parts[0]
			break
		}

		if len(parts) != 2 {
			return fmt.Errorf("invalid field %s", field)
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(strings.ToLower(parts[1]))

		switch key {
		case networkOptName:
			netAttach.Target = value
		case networkOptAlias:
			netAttach.Aliases = append(netAttach.Aliases, value)
		default:
			return fmt.Errorf("invalid field key %s", key)
		}
	}
	n.networkAttachments = append(n.networkAttachments, netAttach)

	return nil
}

// Type returns the type of this option
func (n *NetworkOpt) Type() string {
	return "network"
}

// Value returns the networkopts
func (n *NetworkOpt) Value() []swarm.NetworkAttachmentConfig {
	return n.networkAttachments
}

// String returns the network opts as a string
func (n *NetworkOpt) String() string {
	networks := []string{}
	for _, network := range n.networkAttachments {
		str := fmt.Sprintf("%s:%s", network.Target, strings.Join(network.Aliases, "/"))
		networks = append(networks, str)
	}
	return strings.Join(networks, ",")
}
