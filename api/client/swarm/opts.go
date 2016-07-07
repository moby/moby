package swarm

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/pflag"
)

const (
	defaultListenAddr = "0.0.0.0:2377"
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
	flagExternalCA          = "external-ca"
)

var (
	defaultPolicies = []swarm.Policy{
		{Role: WORKER, Autoaccept: true},
		{Role: MANAGER, Autoaccept: false},
	}
)

type swarmOptions struct {
	autoAccept          AutoAcceptOption
	secret              string
	taskHistoryLimit    int64
	dispatcherHeartbeat time.Duration
	nodeCertExpiry      time.Duration
	externalCA          ExternalCAOption
}

// NodeAddrOption is a pflag.Value for listen and remote addresses
type NodeAddrOption struct {
	addr string
}

// String prints the representation of this flag
func (a *NodeAddrOption) String() string {
	return a.Value()
}

// Set the value for this flag
func (a *NodeAddrOption) Set(value string) error {
	addr, err := opts.ParseTCPAddr(value, a.addr)
	if err != nil {
		return err
	}
	a.addr = addr
	return nil
}

// Type returns the type of this flag
func (a *NodeAddrOption) Type() string {
	return "node-addr"
}

// Value returns the value of this option as addr:port
func (a *NodeAddrOption) Value() string {
	return strings.TrimPrefix(a.addr, "tcp://")
}

// NewNodeAddrOption returns a new node address option
func NewNodeAddrOption(addr string) NodeAddrOption {
	return NodeAddrOption{addr}
}

// NewListenAddrOption returns a NodeAddrOption with default values
func NewListenAddrOption() NodeAddrOption {
	return NewNodeAddrOption(defaultListenAddr)
}

// AutoAcceptOption is a value type for auto-accept policy
type AutoAcceptOption struct {
	values map[string]bool
}

// String prints a string representation of this option
func (o *AutoAcceptOption) String() string {
	keys := []string{}
	for key, value := range o.values {
		keys = append(keys, fmt.Sprintf("%s=%v", strings.ToLower(key), value))
	}
	return strings.Join(keys, ", ")
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

// ExternalCAOption is a Value type for parsing external CA specifications.
type ExternalCAOption struct {
	values []*swarm.ExternalCA
}

// Set parses an external CA option.
func (m *ExternalCAOption) Set(value string) error {
	parsed, err := parseExternalCA(value)
	if err != nil {
		return err
	}

	m.values = append(m.values, parsed)
	return nil
}

// Type returns the type of this option.
func (m *ExternalCAOption) Type() string {
	return "external-ca"
}

// String returns a string repr of this option.
func (m *ExternalCAOption) String() string {
	externalCAs := []string{}
	for _, externalCA := range m.values {
		repr := fmt.Sprintf("%s: %s", externalCA.Protocol, externalCA.URL)
		externalCAs = append(externalCAs, repr)
	}
	return strings.Join(externalCAs, ", ")
}

// Value returns the external CAs
func (m *ExternalCAOption) Value() []*swarm.ExternalCA {
	return m.values
}

// parseExternalCA parses an external CA specification from the command line,
// such as protocol=cfssl,url=https://example.com.
func parseExternalCA(caSpec string) (*swarm.ExternalCA, error) {
	csvReader := csv.NewReader(strings.NewReader(caSpec))
	fields, err := csvReader.Read()
	if err != nil {
		return nil, err
	}

	externalCA := swarm.ExternalCA{
		Options: make(map[string]string),
	}

	var (
		hasProtocol bool
		hasURL      bool
	)

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)

		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid field '%s' must be a key=value pair", field)
		}

		key, value := parts[0], parts[1]

		switch strings.ToLower(key) {
		case "protocol":
			hasProtocol = true
			if strings.ToLower(value) == string(swarm.ExternalCAProtocolCFSSL) {
				externalCA.Protocol = swarm.ExternalCAProtocolCFSSL
			} else {
				return nil, fmt.Errorf("unrecognized external CA protocol %s", value)
			}
		case "url":
			hasURL = true
			externalCA.URL = value
		default:
			externalCA.Options[key] = value
		}
	}

	if !hasProtocol {
		return nil, errors.New("the external-ca option needs a protocol= parameter")
	}
	if !hasURL {
		return nil, errors.New("the external-ca option needs a url= parameter")
	}

	return &externalCA, nil
}

func addSwarmFlags(flags *pflag.FlagSet, opts *swarmOptions) {
	flags.Var(&opts.autoAccept, flagAutoAccept, "Auto acceptance policy (worker, manager or none)")
	flags.StringVar(&opts.secret, flagSecret, "", "Set secret value needed to join a cluster")
	flags.Int64Var(&opts.taskHistoryLimit, flagTaskHistoryLimit, 10, "Task history retention limit")
	flags.DurationVar(&opts.dispatcherHeartbeat, flagDispatcherHeartbeat, time.Duration(5*time.Second), "Dispatcher heartbeat period")
	flags.DurationVar(&opts.nodeCertExpiry, flagCertExpiry, time.Duration(90*24*time.Hour), "Validity period for node certificates")
	flags.Var(&opts.externalCA, flagExternalCA, "Specifications of one or more certificate signing endpoints")
}

func (opts *swarmOptions) ToSpec() swarm.Spec {
	spec := swarm.Spec{}
	if opts.secret != "" {
		spec.AcceptancePolicy.Policies = opts.autoAccept.Policies(&opts.secret)
	} else {
		spec.AcceptancePolicy.Policies = opts.autoAccept.Policies(nil)
	}
	spec.Orchestration.TaskHistoryRetentionLimit = opts.taskHistoryLimit
	spec.Dispatcher.HeartbeatPeriod = uint64(opts.dispatcherHeartbeat.Nanoseconds())
	spec.CAConfig.NodeCertExpiry = opts.nodeCertExpiry
	spec.CAConfig.ExternalCAs = opts.externalCA.Value()
	return spec
}
