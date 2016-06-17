package swarm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/engine-api/types/swarm"
)

const (
	defaultListenAddr        = "0.0.0.0"
	defaultListenPort uint16 = 2377
	// WORKER constant for worker name
	WORKER = "WORKER"
	// MANAGER constant for manager name
	MANAGER = "MANAGER"

	flagAutoAccept          = "auto-accept"
	flagCertExpiry          = "cert-expiry"
	flagDispatcherHeartbeat = "dispatcher-heartbeat"
	flagListenAddr          = "listen-addr"
	flagSecret              = "secret"
	flagTaskHistoryLimit    = "task-history-limit"
)

var (
	defaultPolicies = []swarm.Policy{
		{Role: WORKER, Autoaccept: true},
		{Role: MANAGER, Autoaccept: false},
	}
)

// NodeAddrOption is a pflag.Value for listen and remote addresses
type NodeAddrOption struct {
	addr string
	port uint16
}

// String prints the representation of this flag
func (a *NodeAddrOption) String() string {
	return a.Value()
}

// Set the value for this flag
func (a *NodeAddrOption) Set(value string) error {
	if !strings.Contains(value, ":") {
		a.addr = value
		return nil
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid url, too many colons")
	}

	port, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return fmt.Errorf("Invalid port: %s", parts[1])
	}
	a.port = uint16(port)

	host := parts[0]
	if host != "" {
		a.addr = host
	}
	return nil
}

// Type returns the type of this flag
func (a *NodeAddrOption) Type() string {
	return "node-addr"
}

// Value returns the value of this option as addr:port
func (a *NodeAddrOption) Value() string {
	return fmt.Sprintf("%s:%v", a.addr, a.port)
}

// NewNodeAddrOption returns a new node address option
func NewNodeAddrOption(host string, port uint16) NodeAddrOption {
	return NodeAddrOption{addr: host, port: port}
}

// NewListenAddrOption returns a NodeAddrOption with default values
func NewListenAddrOption() NodeAddrOption {
	return NewNodeAddrOption(defaultListenAddr, defaultListenPort)
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
func (o *AutoAcceptOption) Policies(secret *string) []swarm.Policy {
	policies := []swarm.Policy{}
	for _, p := range defaultPolicies {
		if len(o.values) != 0 {
			p.Autoaccept = o.values[string(p.Role)]
		}
		p.Secret = secret
		policies = append(policies, p)
	}
	return policies
}

// NewAutoAcceptOption returns a new auto-accept option
func NewAutoAcceptOption() AutoAcceptOption {
	return AutoAcceptOption{values: make(map[string]bool)}
}
