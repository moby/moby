package firewallapi

import (
	"net"
)

// Action signifies the nftable action.
type Action string

// Policy is the default nftable policies
type Policy string

// Table refers to Nat, Filter or Mangle.
type Table string

// IPVersion refers to IP version, v4 or v6
type IPVersion string

const (
	// Nat table is used for nat translation rules.
	Nat Table = "nat"
	// Filter table is used for filter rules.
	Filter Table = "filter"
	// Mangle table is used for mangling the packet.
	Mangle Table = "mangle"
)

// VersionTable defines struct with IPVersion
type VersionTable struct {
	Version IPVersion
}

// ChainError is returned to represent errors during nf table operation.
type ChainError struct {
	Chain  string
	Output []byte
}

// FirewallTable represents the operations supported by Linux firewall implementations
type FirewallTable interface {
	// GetTable returns the default implementation for a given
	// firewall type
	GetTable(ipv IPVersion) *VersionTable
	NewChain(name string, table Table, hairpinMode bool) (FirewallChain, error)
	FlushChain(table Table, name string) error
	LoopbackByVersion() string
	ProgramChain(c FirewallChain, bridgeName string, hairpinMode, enable bool) error
	RemoveExistingChain(name string, table Table) error
	ProgramRule(table Table, chain string, action Action, args []string) error
	Exists(table Table, chain string, rule ...string) bool
	ExistsNative(table Table, chain string, rule ...string) bool
	exists(native bool, table Table, chain string, rule ...string) bool
	existsRaw(table Table, chain string, rule ...string) bool
	Raw(args ...string) ([]byte, error)
	raw(args ...string) ([]byte, error)
	RawCombinedOutput(args ...string) error
	RawCombinedOutputNative(args ...string) error
	ExistChain(chain string, table Table) bool
	SetDefaultPolicy(table Table, chain string, policy Policy) error
	DeleteRule(version IPVersion, table Table, chain string, rule ...string) error
	AddReturnRule(chain string) error
	EnsureJumpRule(fromChain, toChain string) error
	EnsureJumpRuleForIface(fromChain, toChain, iface string) error
	EnsureAcceptRule(chain string) error
	EnsureAcceptRuleForIface(chain, iface string) error
	EnsureDropRule(chain string) error
	EnsureDropRuleForIface(chain, iface string) error
	EnsureReturnRule(table Table, chain string) error
	EnsureLocalMasquerade(table Table, fromChain, toChain string) error
	EnsureLocalMasqueradeForIface(table Table, iface string) error
	AddJumpRuleForIP(table Table, fromChain, toChain, ipaddr string)
	//AddDNATwithPort sets up a DNAT rule which forward all traffic on that port
	AddDNATwithPort(table Table, chain, dstIP, dstPort, proto, natIP string)
	//AddSNATwithPort sets up a SNAT rule which masquerades all traffic on that port
	AddSNATwithPort(table Table, chain, srcIP, srcPort, proto, natPort string)
	GetInsertAction() string
	GetAppendAction() string
	GetDeleteAction() string
	GetDropPolicy() string
	GetAcceptPolicy() string
}

// FirewallChain represents the operations supported by iptables/nftables chains
type FirewallChain interface {
	Forward(action Action, ip net.IP, port int, proto, destAddr string, destPort int, bridgeName string) error
	Link(action Action, ip1, ip2 net.IP, port int, proto string, bridgeName string) error
	DeleteRule(version IPVersion, table Table, chain string, rule ...string) error
	Prerouting(action Action, args ...string) error
	Output(action Action, args ...string) error
	Remove() error
	GetName() string
	GetTable() Table
	SetTable(Table)
	GetHairpinMode() bool
	GetFirewallTable() FirewallTable
}
