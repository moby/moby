package nftables

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/firewalld"
	"github.com/sirupsen/logrus"
)

// Action signifies the nftable action.
type Action = firewallapi.Action

// Policy is the default nftable policies
type Policy = firewallapi.Policy

// Table refers to Nat, Filter or Mangle.
type Table = firewallapi.Table

// IPVersion refers to IP version, v4 or v6
type IPVersion = firewallapi.IPVersion

const (
	// Append appends the rule at the end of the chain.
	Append Action = "add rule"
	// Delete deletes the rule from the chain.
	Delete Action = "delete rule"
	// Insert inserts the rule at the top of the chain.
	Insert Action = "insert rule"
	// Nat table is used for nat translation rules.
	Nat firewallapi.Table = firewallapi.Nat
	// Filter table is used for filter rules.
	Filter firewallapi.Table = firewallapi.Filter
	// Mangle table is used for mangling the packet.
	Mangle firewallapi.Table = firewallapi.Mangle
	// Drop is the default nftables DROP policy
	Drop Policy = "drop"
	// Accept is the default nftables ACCEPT policy
	Accept Policy = "accept"
	// IPv4 sets the correct string for inet
	// inet is both version 4 and 6
	// "ip" and "ip6" can be used to segregate, but unlike iptables
	// nftable doesn't need to differentiate between binaries, so
	// this distinction is kept only for compatibility with drivers
	// ip is version 4
	IPv4 IPVersion = "ip"
	// IPv6 sets the correct string. ip6 is version 6
	IPv6 IPVersion = "ip6"
)

var (
	nftablesPath   string
	bestEffortLock sync.Mutex
	// ErrNftablesNotFound is returned when the rule is not found.
	ErrNftablesNotFound = errors.New("Nftables not found")
	initOnce            sync.Once
)

//NFTable is a basic mapping to implement the interface
type NFTable struct {
	firewallapi.FirewallTable
	Version IPVersion
}

// ChainInfo defines the nftables chain.
type ChainInfo struct {
	Name          string
	Table         Table
	HairpinMode   bool
	FirewallTable NFTable
}

// ChainError is returned to represent errors during nf table operation.
type ChainError struct {
	Chain  string
	Output []byte
}

func (e ChainError) Error() string {
	return fmt.Sprintf("Error nftables %s: %s", e.Chain, string(e.Output))
}

func probe() {
	path, err := exec.LookPath("nft")
	if err != nil {
		logrus.Debugf("Failed to find nftables: %v", err)
		return
	}
	if out, err := exec.Command(path, "-n", "list", "table", "nat").CombinedOutput(); err != nil {
		logrus.Warnf("Running nft -n list table nat failed with message: `%s`, error: %v", strings.TrimSpace(string(out)), err)
	}
}

func initFirewalld() {
	if err := firewalld.FirewalldInit(); err != nil {
		logrus.Debugf("Fail to initialize firewalld: %v, using raw nftables instead", err)
	}
}

func detectNftables() {
	path, err := exec.LookPath("nft")
	if err != nil {
		return
	}
	nftablesPath = path
}

func initDependencies() {
	probe()
	initFirewalld()
	detectNftables()
}

//InitCheck puts the module together
func InitCheck() error {
	initOnce.Do(initDependencies)

	if nftablesPath == "" {
		return ErrNftablesNotFound
	}
	return nil
}

// GetTable returns an instance of NFTable with specified version
func GetTable(version IPVersion) firewallapi.FirewallTable {
	return &NFTable{Version: version}
}

// NewChain adds a new chain to ip table.
func (nftable NFTable) NewChain(name string, table Table, hairpinMode bool) (firewallapi.FirewallChain, error) {
	c := &ChainInfo{
		Name:          name,
		Table:         table,
		HairpinMode:   hairpinMode,
		FirewallTable: nftable,
	}

	if string(c.GetTable()) == "" {
		c.SetTable(Table(firewallapi.Filter))
	}

	// Add chain if it doesn't exist
	if _, err := nftable.Raw("-n", "list", "table", string(c.FirewallTable.Version), string(c.GetTable()), c.GetName()); err != nil {
		if output, err := nftable.raw("add", "chain", string(c.FirewallTable.Version), string(c.GetTable()), c.GetName()); err != nil {
			return nil, err
		} else if len(output) != 0 {
			return nil, fmt.Errorf("Could not create %s/%s/%s chain: %s", c.FirewallTable.Version, c.GetTable(), c.GetName(), output)
		}
	}
	return c, nil
}

func (nftable NFTable) FlushChain(table Table, name string) error {
	name = strings.ToLower(name)
	if _, err := nftable.Raw("flush", "chain", "ip", string(table)); err != nil {
		return err
	}

	return nil
}

// LoopbackByVersion returns loopback address by version
func (nftable NFTable) LoopbackByVersion() string {
	if nftable.Version == IPv6 {
		return "::1/128"
	}
	return "127.0.0.0/8"
}

// ProgramChain is used to add rules to a chain
func (nftable NFTable) ProgramChain(c firewallapi.FirewallChain, bridgeName string, hairpinMode, enable bool) error {
	if c.GetName() == "" {
		return errors.New("Could not program chain, missing chain name")
	}

	// Either add or remove the interface from the firewalld zone
	if firewalld.FirewalldRunning {
		if enable {
			if err := firewalld.AddInterfaceFirewalld(bridgeName); err != nil {
				return err
			}
		} else {
			if err := firewalld.DelInterfaceFirewalld(bridgeName); err != nil {
				return err
			}
		}
	}

	switch c.GetTable() {
	case Nat:
		// daddr type 2 is local
		preroute := []string{
			"fib", "daddr",
			"type", "2",
			"jump", c.GetName()}
		if !nftable.Exists(Nat, "prerouting", preroute...) && enable {
			if err := c.Prerouting(Append, preroute...); err != nil {
				return fmt.Errorf("Failed to inject %s in prerouting chain: %s", c.GetName(), err)
			}
		} else if nftable.Exists(Nat, "prerouting", preroute...) && !enable {
			if err := c.DeleteRule(nftable.Version, c.GetTable(), "prerouting", preroute...); err != nil {
				return fmt.Errorf("Failed to remove %s in prerouting chain: %s", c.GetName(), err)
			}
		}
		output := []string{
			"fib", "daddr",
			"type", "2",
			"jump", c.GetName()}
		if !hairpinMode {
			output = append([]string{"daddr", "!=", nftable.LoopbackByVersion()}, output...)
		}
		if !nftable.Exists(Nat, "output", output...) && enable {
			if err := c.Output(Append, output...); err != nil {
				return fmt.Errorf("Failed to inject %s in output chain: %s", c.GetName(), err)
			}
		} else if nftable.Exists(Nat, "output", output...) && !enable {
			if err := c.DeleteRule(nftable.Version, c.GetTable(), "output", output...); err != nil {
				return fmt.Errorf("Failed to inject %s in output chain: %s", c.GetName(), err)
			}
		}
	case Filter:
		if bridgeName == "" {
			return fmt.Errorf("Could not program chain %s/%s, missing bridge name",
				c.GetTable(), c.GetName())
		}
		link := []string{
			"oifname", bridgeName,
			"jump", c.GetName()}
		if !nftable.Exists(Filter, "forward", link...) && enable {
			insert := append([]string{string(Insert), "forward"}, link...)
			if output, err := nftable.Raw(insert...); err != nil {
				return err
			} else if len(output) != 0 {
				return fmt.Errorf("Could not create linking rule to %s/%s: %s", c.GetTable(), c.GetName(), output)
			}
		} else if nftable.Exists(Filter, "forward", link...) && !enable {
			if err := c.DeleteRule(nftable.Version, c.GetTable(), "forward", link...); err != nil {
				return fmt.Errorf("Could not delete linking rule from %s/%s: %s", c.GetTable(), c.GetName(), err)
			}

		}
		establish := []string{
			"oifname", bridgeName,
			"ct", "state", "related, established",
			"accept"}
		if !nftable.Exists(Filter, "forward", establish...) && enable {
			insert := append([]string{string(Insert), "forward"}, establish...)
			if output, err := nftable.Raw(insert...); err != nil {
				return err
			} else if len(output) != 0 {
				return fmt.Errorf("Could not create establish rule to %s: %s", c.GetTable(), output)
			}
		} else if nftable.Exists(Filter, "forward", establish...) && !enable {
			if err := c.DeleteRule(nftable.Version, c.GetTable(), "forward", establish...); err != nil {
				return fmt.Errorf("Could not delete establish rule from %s: %s", c.GetTable(), err)
			}
		}
	}
	return nil
}

// RemoveExistingChain removes existing chain from the table.
func (nftable NFTable) RemoveExistingChain(name string, table Table) error {
	c := &ChainInfo{
		Name:          name,
		Table:         table,
		FirewallTable: nftable,
	}
	if string(c.GetTable()) == "" {
		c.SetTable(Filter)
	}
	return c.Remove()
}

// Forward adds forwarding rule to 'filter' table and corresponding nat rule to 'nat' table.
func (c ChainInfo) Forward(action Action, ip net.IP, port int, proto, destAddr string, destPort int, bridgeName string) error {

	nftable := GetTable(c.FirewallTable.Version)
	daddr := ip.String()
	var args []string
	if ip.IsUnspecified() {
		// nftables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
		// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
		// value" by nftables.
		args = []string{
			proto,
			"dport", strconv.Itoa(port),
			"dnat",
			"to", net.JoinHostPort(destAddr, strconv.Itoa(destPort))}
	} else {
		args = []string{
			"ip", "daddr", daddr,
			proto,
			"dport", strconv.Itoa(port),
			"dnat",
			"to", net.JoinHostPort(destAddr, strconv.Itoa(destPort))}
	}

	if !c.HairpinMode {
		args = append([]string{"iifname", "!=", bridgeName}, args...)
	}
	if err := nftable.ProgramRule(Nat, c.GetName(), action, args); err != nil {
		return err
	}

	if ip.IsUnspecified() {
		// nftables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
		// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
		// value" by nftables.
		args = []string{
			"iifname", "!=", bridgeName,
			"oifname", bridgeName,
			proto,
			"dport", strconv.Itoa(port),
			"accept"}
	} else {
		args = []string{
			"iifname", "!=", bridgeName,
			"oifname", bridgeName,
			"ip", "daddr", daddr,
			proto,
			"dport", strconv.Itoa(port),
			"accept"}
	}

	if err := nftable.ProgramRule(Filter, c.GetName(), action, args); err != nil {
		return err
	}

	args = []string{
		"ip", "saddr", destAddr,
		"ip", "daddr", destAddr,
		proto,
		"dport", strconv.Itoa(port),
		"masquerade"}

	if err := nftable.ProgramRule(Nat, "postrouting", action, args); err != nil {
		return err
	}

	return nil
}

// Link adds reciprocal ACCEPT rule for two supplied IP addresses.
// Traffic is allowed from ip1 to ip2 and vice-versa
func (c ChainInfo) Link(action Action, ip1, ip2 net.IP, port int, proto string, bridgeName string) error {
	nftable := GetTable(c.FirewallTable.Version)
	// forward
	args := []string{
		"iifname", bridgeName,
		"oifname", bridgeName,
		"ip", "saddr", ip1.String(),
		"ip", "daddr", ip2.String(),
		proto, "dport", strconv.Itoa(port),
		"accept"}

	if err := nftable.ProgramRule(Filter, c.GetName(), action, args); err != nil {
		return err
	}
	// reverse
	args = []string{
		"iifname", bridgeName,
		"oifname", bridgeName,
		"ip", "saddr", ip2.String(),
		"ip", "daddr", ip1.String(),
		proto, "sport", strconv.Itoa(port),
		"accept"}
	return nftable.ProgramRule(Filter, c.GetName(), action, args)
}

//DeleteRule passes down to a raw level since it's more complex in NFTables
func (c ChainInfo) DeleteRule(version IPVersion, table Table, chain string, rule ...string) error {
	chain = strings.ToLower(chain)

	return DeleteRule(version, table, chain, rule...)
}

//DeleteRule passes down to a raw level since it's more complex in NFTables
func (nftable NFTable) DeleteRule(version IPVersion, table Table, chain string, rule ...string) error {
	chain = strings.ToLower(chain)

	return DeleteRule(version, table, chain, rule...)
}

//DeleteRule passes down to a raw level since it's more complex in NFTables
func DeleteRule(version IPVersion, table Table, chain string, rule ...string) error {
	chain = strings.ToLower(chain)

	bestEffortLock.Lock()
	defer bestEffortLock.Unlock()

	path := nftablesPath
	commandName := "nft"

	findRuleArgs := []string{"-n", "-a", "list", "chain", string(version), string(table), chain}
	handleMatcher := regexp.MustCompile(`#\shandle\s(\d+)`)
	var handle string

	findRule, err := exec.Command(path, findRuleArgs...).CombinedOutput()

	ruleStr := strings.Split(string(findRule), "\n")

	for _, d := range ruleStr {
		if strings.Contains(d, strings.Join(rule, " ")) {
			handle = handleMatcher.FindStringSubmatch(d)[1]
		}
	}

	args := []string{string(Delete), string(version), string(table), chain, "handle", handle}

	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft failed: %s %v: %s (%s)", commandName, strings.Join(args, " "), output, err)
	}

	return nil
}

// ProgramRule adds the rule specified by args only if the
// rule is not already present in the chain. Reciprocally,
// it removes the rule only if present.
func (nftable NFTable) ProgramRule(table Table, chain string, action Action, args []string) error {
	chain = strings.ToLower(chain)

	if nftable.Exists(table, chain, args...) != (action == Delete) {
		return nil
	}
	return nftable.RawCombinedOutput(append([]string{string(action), string(nftable.Version), string(table), chain}, args...)...)
}

// Prerouting adds linking rule to nat/PREROUTING chain.
func (c ChainInfo) Prerouting(action Action, args ...string) error {
	nftable := GetTable(c.FirewallTable.Version)
	a := []string{string(action), string(c.FirewallTable.Version), string(Nat), "prerouting"}
	if len(args) > 0 {
		a = append(a, args...)
	}
	if output, err := nftable.Raw(a...); err != nil {
		return err
	} else if len(output) != 0 {
		return ChainError{Chain: "prerouting", Output: output}
	}
	return nil
}

// ForwardChain adds linking rule to a forward chain.
func (c ChainInfo) ForwardChain(action Action, args ...string) error {
	nftable := GetTable(c.FirewallTable.Version)
	a := []string{string(action), string(c.FirewallTable.Version), string(c.GetTable()), "forward"}
	if len(args) > 0 {
		a = append(a, args...)
	}
	if output, err := nftable.Raw(a...); err != nil {
		return err
	} else if len(output) != 0 {
		return ChainError{Chain: "forward", Output: output}
	}
	return nil
}

// Output adds linking rule to an OUTPUT chain.
func (c ChainInfo) Output(action Action, args ...string) error {
	nftable := GetTable(c.FirewallTable.Version)
	a := []string{string(action), string(c.FirewallTable.Version), string(c.GetTable()), "output"}
	if len(args) > 0 {
		a = append(a, args...)
	}
	if output, err := nftable.Raw(a...); err != nil {
		return err
	} else if len(output) != 0 {
		return ChainError{Chain: "output", Output: output}
	}
	return nil
}

// Remove removes the chain.
func (c ChainInfo) Remove() error {
	nftable := GetTable(c.FirewallTable.Version)

	nftable.Raw("flush", "chain", string(c.FirewallTable.Version), c.GetName())
	nftable.Raw("delete", "chain", string(c.FirewallTable.Version), string(c.GetTable()), c.GetName())
	return nil
}

// Exists checks if a rule exists
func (nftable NFTable) Exists(table Table, chain string, rule ...string) bool {
	chain = strings.ToLower(chain)

	return nftable.exists(false, table, chain, rule...)
}

// ExistsNative behaves as Exists with the difference it
// will always invoke `nft` binary.
func (nftable NFTable) ExistsNative(table Table, chain string, rule ...string) bool {
	chain = strings.ToLower(chain)

	return nftable.exists(true, table, chain, rule...)
}

func (nftable NFTable) exists(native bool, table Table, chain string, rule ...string) bool {
	chain = strings.ToLower(chain)

	f := nftable.Raw
	if native {
		f = nftable.raw
	}

	if string(table) == "" {
		table = Filter
	}

	if err := InitCheck(); err != nil {
		// The exists() signature does not allow us to return an error, but at least
		// we can skip the (likely invalid) exec invocation.
		return false
	}

	findRuleArgs := []string{"-n", "-a", "list", "chain", string(nftable.Version), string(table), chain}
	handleMatcher := regexp.MustCompile(`#\shandle\s(\d+)`)
	var handle string

	findRule, _ := f(findRuleArgs...)

	ruleStr := strings.Split(string(findRule), "\n")

	for _, d := range ruleStr {
		if strings.Contains(d, strings.Join(rule, " ")) {
			handle = handleMatcher.FindStringSubmatch(d)[1]
		}
	}
	return handle == ""
}

func (nftable NFTable) existsRaw(table Table, chain string, rule ...string) bool {
	chain = strings.ToLower(chain)

	path := nftablesPath
	if nftable.Version == IPv6 {
		// Do somethng here
	}
	ruleString := fmt.Sprintf("%s %s\n", chain, strings.Join(rule, " "))
	existingRules, _ := exec.Command(path, "-t", string(table), "-S", chain).Output()

	return strings.Contains(string(existingRules), ruleString)
}

// Raw calls 'nft' system command, passing supplied arguments.
func (nftable NFTable) Raw(args ...string) ([]byte, error) {
	var firewalldTable firewalld.IPV
	if nftable.Version == IPv4 {
		firewalldTable = firewalld.Iptables
	} else {
		firewalldTable = firewalld.IP6Tables
	}
	if firewalld.FirewalldRunning {
		output, err := firewalld.Passthrough(firewalldTable, args...)
		if err == nil || !strings.Contains(err.Error(), "was not provided by any .service files") {
			return output, err
		}
	}
	return nftable.raw(args...)
}

func (nftable NFTable) raw(args ...string) ([]byte, error) {
	if err := InitCheck(); err != nil {
		return nil, err
	}
	bestEffortLock.Lock()
	defer bestEffortLock.Unlock()

	path := nftablesPath
	commandName := "nft"

	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nft failed: %s %v: %s (%s)", commandName, strings.Join(args, " "), output, err)
	}

	return output, err
}

// RawCombinedOutput internally calls the Raw function and returns a non nil
// error if Raw returned a non nil error or a non empty output
func (nftable NFTable) RawCombinedOutput(args ...string) error {
	if output, err := nftable.Raw(args...); err != nil || len(output) != 0 {
		return fmt.Errorf("%s (%v)", string(output), err)
	}
	return nil
}

// RawCombinedOutputNative behave as RawCombinedOutput with the difference it
// will always invoke `nft` binary
func (nftable NFTable) RawCombinedOutputNative(args ...string) error {
	if output, err := nftable.raw(args...); err != nil || len(output) != 0 {
		return fmt.Errorf("%s (%v)", string(output), err)
	}
	return nil
}

// ExistChain checks if a chain exists
func (nftable NFTable) ExistChain(chain string, table Table) bool {
	chain = strings.ToLower(chain)

	if _, err := nftable.Raw("-n", "list", "table", string(nftable.Version), string(table)); err == nil {
		return true
	}
	return false
}

// GetVersion reads the nft version numbers during initialization
func GetVersion() (major, minor, micro int, err error) {
	out, err := exec.Command(nftablesPath, "--version").CombinedOutput()
	if err == nil {
		major, minor, micro = parseVersionNumbers(string(out))
	}
	return
}

// SetDefaultPolicy sets the passed default policy for the table/chain
func (nftable NFTable) SetDefaultPolicy(table Table, chain string, policy Policy) error {
	chain = strings.ToLower(chain)
	if err := nftable.RawCombinedOutput(chain, string(nftable.Version), string(table),
		fmt.Sprintf("'{ policy %s ; }'", strings.ToLower(string(policy)))); err != nil {
		return fmt.Errorf("setting default policy to %v in %v chain failed: %v", policy, chain, err)
	}
	return nil
}

func parseVersionNumbers(input string) (major, minor, micro int) {
	re := regexp.MustCompile(`v\d*.\d*.\d*`)
	line := re.FindString(input)
	fmt.Sscanf(line, "v%d.%d.%d", &major, &minor, &micro)
	return
}

// AddReturnRule adds a return rule for the chain in the filter table
func (nftable NFTable) AddReturnRule(chain string) error {
	chain = strings.ToLower(chain)
	var (
		table = Filter
		args  = []string{"return"}
	)

	if nftable.Exists(table, chain, args...) {
		return nil
	}

	err := nftable.RawCombinedOutput(append([]string{string(Append), string(nftable.Version), string(table), chain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to add return rule in %s chain: %s", chain, err.Error())
	}

	return nil
}

// EnsureJumpRule ensures the jump rule is on top
func (nftable NFTable) EnsureJumpRule(fromChain, toChain string) error {
	fromChain = strings.ToLower(fromChain)
	toChain = strings.ToLower(toChain)

	var (
		table = Filter
		args  = []string{"jump", toChain}
	)

	if nftable.Exists(table, fromChain, args...) {
		err := nftable.DeleteRule(nftable.Version, "filter", fromChain, args...)
		if err != nil {
			return fmt.Errorf("unable to remove jump to %s rule in %s chain: %s", toChain, fromChain, err.Error())
		}
	}

	err := nftable.RawCombinedOutput(append([]string{"insert", "rule", "ip", "filter", fromChain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to insert jump to %s rule in %s chain: %s", toChain, fromChain, err.Error())
	}

	return nil
}

// EnsureAcceptRule is a re-implementation of some hardcoded logic to make sure it's not already in the ruleset
func (nftable NFTable) EnsureAcceptRule(chain string) error {
	chain = strings.ToLower(chain)

	var (
		table = Filter
		args  = []string{"accept"}
	)

	if nftable.Exists(table, chain, args...) {
		err := nftable.DeleteRule(nftable.Version, "filter", chain, args...)
		if err != nil {
			return fmt.Errorf("unable to remove accept rule in %s chain: %s", chain, err.Error())
		}
	}

	err := nftable.RawCombinedOutput(append([]string{"add", "rule", "ip", "filter", chain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to insert accept rule in %s chain: %s", chain, err.Error())
	}

	return nil
}

// EnsureAcceptRuleForIface is a re-implementation of some hardcoded logic to make sure it's not already in the ruleset
func (nftable NFTable) EnsureAcceptRuleForIface(chain, iface string) error {
	chain = strings.ToLower(chain)

	var (
		table = Filter
		args  = []string{"oifname", iface, "accept"}
	)

	if nftable.Exists(table, chain, args...) {
		err := nftable.DeleteRule(nftable.Version, "filter", chain, args...)
		if err != nil {
			return fmt.Errorf("unable to remove accept rule in %s chain: %s", chain, err.Error())
		}
	}

	err := nftable.RawCombinedOutput(append([]string{"add", "rule", string(nftable.Version), "filter", chain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to insert accept rule in %s chain: %s", chain, err.Error())
	}

	return nil
}

// EnsureDropRule is a re-implementation of some hardcoded logic to make sure it's not already in the ruleset
func (nftable NFTable) EnsureDropRule(chain string) error {
	chain = strings.ToLower(chain)

	var (
		table = Filter
		args  = []string{"drop"}
	)

	if nftable.Exists(table, chain, args...) {
		err := nftable.DeleteRule(nftable.Version, "filter", chain, args...)
		if err != nil {
			return fmt.Errorf("unable to remove drop rule in %s chain: %s", chain, err.Error())
		}
	}

	err := nftable.RawCombinedOutput(append([]string{"add", "rule", string(nftable.Version), "filter", chain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to insert drop rule in %s chain: %s", chain, err.Error())
	}

	return nil
}

// EnsureReturnRule makes sure that that's a return rule at the end of the table
func (nftable NFTable) EnsureReturnRule(table Table, chain string) error {
	chain = strings.ToLower(chain)

	args := []string{"return"}

	if !nftable.Exists(table, chain, args...) {
		err := nftable.RawCombinedOutput(append([]string{"add", "rule", string(nftable.Version), string(table), chain}, args...)...)
		if err != nil {
			return fmt.Errorf("unable to ensure jump rule in %s chain: %s", chain, err.Error())
		}
	}

	return nil
}

// EnsureLocalMasquerade ensures the jump rule is on top
func (nftable NFTable) EnsureLocalMasquerade(table Table, fromChain, toChain string) error {
	var (
		args = []string{"fib", "daddr", "type", "local", "jump", toChain}
	)

	if !nftable.Exists(table, fromChain, args...) {
		if err := nftable.RawCombinedOutput(append([]string{"insert", "rule", string(nftable.Version), string(table), fromChain}, args...)...); err != nil {
			return fmt.Errorf("failed to add jump rule in %s to ingress chain: %v", toChain, err)
		}
	}

	return nil
}

// EnsureLocalMasqueradeForIface ensures the jump rule is on top
func (nftable NFTable) EnsureLocalMasqueradeForIface(table Table, iface string) error {
	var (
		args = []string{"oifname", iface, "fib", "saddr", "type", "local", "masquerade"}
	)

	if !nftable.Exists(table, "POSTROUTING", args...) {
		if err := nftable.RawCombinedOutput(append([]string{"insert", "rule", string(nftable.Version), string(table)}, args...)...); err != nil {
			return fmt.Errorf("failed to add ingress localhost POSTROUTING rule for %s: %v", iface, err)
		}
	}

	return nil
}

// EnsureDropRuleForIface is a re-implementation of some hardcoded logic to make sure it's not already in the ruleset
func (nftable NFTable) EnsureDropRuleForIface(chain, iface string) error {
	chain = strings.ToLower(chain)

	var (
		table = Filter
		args  = []string{"oifname", iface, "drop"}
	)

	if nftable.Exists(table, chain, args...) {
		err := nftable.DeleteRule(nftable.Version, "filter", chain, args...)
		if err != nil {
			return fmt.Errorf("unable to remove drop rule in %s chain: %s", chain, err.Error())
		}
	}

	err := nftable.RawCombinedOutput(append([]string{"add", "rule", "ip", "filter", chain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to insert drop rule in %s chain: %s", chain, err.Error())
	}

	return nil
}

// EnsureJumpRuleForIface ensures the jump rule is on top
func (nftable NFTable) EnsureJumpRuleForIface(fromChain, toChain, iface string) error {
	fromChain = strings.ToLower(fromChain)
	toChain = strings.ToLower(toChain)

	var (
		table = Filter
		args  = []string{"oifname", iface, "jump", toChain}
	)

	if nftable.Exists(table, fromChain, args...) {
		err := nftable.DeleteRule(nftable.Version, "filter", fromChain, args...)
		if err != nil {
			return fmt.Errorf("unable to remove jump to %s rule in %s chain: %s", toChain, fromChain, err.Error())
		}
	}

	err := nftable.RawCombinedOutput(append([]string{"insert", "rule", "ip", "filter", fromChain}, args...)...)
	if err != nil {
		return fmt.Errorf("unable to insert jump to %s rule in %s chain: %s", toChain, fromChain, err.Error())
	}

	return nil
}

//AddJumpRuleForIP ensures that there is a jump rule for a given IP at the top of the chain
func (nftable NFTable) AddJumpRuleForIP(table Table, fromChain, toChain, ipaddr string) {
	if nftable.Exists(table, fromChain, "ip", "addr", ipaddr, "jump", toChain) {
		nftable.RawCombinedOutputNative("flush", "chain", "ip", string(table), toChain)
	} else {
		nftable.RawCombinedOutputNative("add", "chain", "ip", string(table), toChain)
		nftable.RawCombinedOutputNative("-t", string(table), "-I", fromChain, "-d", ipaddr, "-j", toChain)
	}
}

//AddDNAT adds a dnat rule witth a port
func (nftable NFTable) AddDNATwithPort(table Table, chain, dstIP, dstPort, proto, natIP string) {
	rule := []string{"insert", "rule", "ip", string(table), chain, "ip", "daddr", dstIP, proto, "dport", dstPort, "dnat", "to", natIP}
	if nftable.RawCombinedOutputNative(rule...) != nil {
		logrus.Errorf("set up rule failed, %v", rule)
	}
}

//AddSNAT adds a snat rule with a port
func (nftable NFTable) AddSNATwithPort(table Table, chain, srcIP, srcPort, proto, natPort string) {
	rule := []string{"insert", "rule", "ip", string(table), chain, "ip", "saddr", srcIP, proto, "sport", srcPort, "snat", "to", ":" + natPort}
	if nftable.RawCombinedOutputNative(rule...) != nil {
		logrus.Errorf("set up rule failed, %v", rule)
	}
}

func (nftable NFTable) GetInsertAction() string {
	return string(Insert)
}

func (nftable NFTable) GetAppendAction() string {
	return string(Append)
}

func (nftable NFTable) GetDeleteAction() string {
	return string(Delete)
}

func (nftable NFTable) GetDropPolicy() string {
	return string(Drop)
}

func (nftable NFTable) GetAcceptPolicy() string {
	return string(Accept)
}

func (c ChainInfo) GetName() string {
	return c.Name
}

func (c ChainInfo) GetTable() Table {
	return c.Table
}

func (c ChainInfo) SetTable(t Table) {
	c.Table = t
}

func (c ChainInfo) GetHairpinMode() bool {
	return c.HairpinMode
}

func (c ChainInfo) GetFirewallTable() firewallapi.FirewallTable {
	return c.FirewallTable
}
