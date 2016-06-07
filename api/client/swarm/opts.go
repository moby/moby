package swarm

import (
	"fmt"
	"strings"

	"github.com/docker/engine-api/types/swarm"
)

const (
	defaultListenAddr = "0.0.0.0:2377"
	// WORKER constant for worker name
	WORKER = "WORKER"
	// MANAGER constant for manager name
	MANAGER = "MANAGER"
)

var (
	defaultPolicy = swarm.Policy{Role: WORKER, Autoaccept: true}
)

// NodeAddrOption is a pflag.Value for listen and remote addresses
type NodeAddrOption struct {
	addr string
}

// String prints the representation of this flag
func (a *NodeAddrOption) String() string {
	return a.addr
}

// Set the value for this flag
func (a *NodeAddrOption) Set(value string) error {
	if !strings.Contains(value, ":") {
		return fmt.Errorf("Invalud url, a host and port are required")
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return fmt.Errorf("Invalud url, too many colons")
	}

	a.addr = value
	return nil
}

// Type returns the type of this flag
func (a *NodeAddrOption) Type() string {
	return "node-addr"
}

// NewNodeAddrOption returns a new node address option
func NewNodeAddrOption() NodeAddrOption {
	return NodeAddrOption{addr: defaultListenAddr}
}

// AutoAcceptOption is a value type for auto-accept policy
type AutoAcceptOption struct {
	values map[string]bool
}

// String prints a string representation of this option
func (o *AutoAcceptOption) String() string {
	keys := []string{}
	for key := range o.values {
		keys = append(keys, key)
	}
	return strings.Join(keys, " ")
}

// Set sets a new value on this option
func (o *AutoAcceptOption) Set(value string) error {
	value = strings.ToUpper(value)
	switch value {
	case "", "NONE":
		if accept, ok := o.values[WORKER]; ok && accept {
			return fmt.Errorf("value NONE is incompatible with %s", WORKER)
		}
		if accept, ok := o.values[MANAGER]; ok && accept {
			return fmt.Errorf("value NONE is incompatible with %s", MANAGER)
		}
		o.values[WORKER] = false
		o.values[MANAGER] = false
	case WORKER, MANAGER:
		if accept, ok := o.values[value]; ok && !accept {
			return fmt.Errorf("value NONE is incompatible with %s", value)
		}
		o.values[value] = true
	default:
		return fmt.Errorf("must be one of %s, %s, NONE", WORKER, MANAGER)
	}

	return nil
}

// Type returns the type of this option
func (o *AutoAcceptOption) Type() string {
	return "auto-accept"
}

// Policies returns a representation of this option for the api
func (o *AutoAcceptOption) Policies(secret string) []swarm.Policy {
	policies := []swarm.Policy{}

	if len(o.values) == 0 {
		return append(policies, defaultPolicy)
	}

	for role, enabled := range o.values {
		policies = append(policies, swarm.Policy{
			Role:       role,
			Autoaccept: enabled,
			Secret:     secret,
		})
	}
	return policies
}

// NewAutoAcceptOption returns a new auto-accept option
func NewAutoAcceptOption() AutoAcceptOption {
	return AutoAcceptOption{values: make(map[string]bool)}
}
