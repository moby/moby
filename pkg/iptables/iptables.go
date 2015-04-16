package iptables

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type Action string
type Table string

const (
	Append Action = "-A"
	Delete Action = "-D"
	Insert Action = "-I"
	Nat    Table  = "nat"
	Filter Table  = "filter"
	Mangle Table  = "mangle"
)

var (
	iptablesPath        string
	supportsXlock       = false
	ErrIptablesNotFound = errors.New("Iptables not found")
)

type Chain struct {
	Name   string
	Bridge string
	Table  Table
}

type ChainError struct {
	Chain  string
	Output []byte
}

func (e *ChainError) Error() string {
	return fmt.Sprintf("Error iptables %s: %s", e.Chain, string(e.Output))
}

func initCheck() error {

	if iptablesPath == "" {
		path, err := exec.LookPath("iptables")
		if err != nil {
			return ErrIptablesNotFound
		}
		iptablesPath = path
		supportsXlock = exec.Command(iptablesPath, "--wait", "-L", "-n").Run() == nil
	}
	return nil
}

func NewChain(name, bridge string, table Table) (*Chain, error) {
	c := &Chain{
		Name:   name,
		Bridge: bridge,
		Table:  table,
	}

	if string(c.Table) == "" {
		c.Table = Filter
	}

	// Add chain if it doesn't exist
	if _, err := Raw("-t", string(c.Table), "-n", "-L", c.Name); err != nil {
		if output, err := Raw("-t", string(c.Table), "-N", c.Name); err != nil {
			return nil, err
		} else if len(output) != 0 {
			return nil, fmt.Errorf("Could not create %s/%s chain: %s", c.Table, c.Name, output)
		}
	}

	switch table {
	case Nat:
		preroute := []string{
			"-m", "addrtype",
			"--dst-type", "LOCAL"}
		if !Exists(Nat, "PREROUTING", preroute...) {
			if err := c.Prerouting(Append, preroute...); err != nil {
				return nil, fmt.Errorf("Failed to inject docker in PREROUTING chain: %s", err)
			}
		}
		output := []string{
			"-m", "addrtype",
			"--dst-type", "LOCAL",
			"!", "--dst", "127.0.0.0/8"}
		if !Exists(Nat, "OUTPUT", output...) {
			if err := c.Output(Append, output...); err != nil {
				return nil, fmt.Errorf("Failed to inject docker in OUTPUT chain: %s", err)
			}
		}
	case Filter:
		link := []string{
			"-o", c.Bridge,
			"-j", c.Name}
		if !Exists(Filter, "FORWARD", link...) {
			insert := append([]string{string(Insert), "FORWARD"}, link...)
			if output, err := Raw(insert...); err != nil {
				return nil, err
			} else if len(output) != 0 {
				return nil, fmt.Errorf("Could not create linking rule to %s/%s: %s", c.Table, c.Name, output)
			}
		}
	}
	return c, nil
}

func RemoveExistingChain(name string, table Table) error {
	c := &Chain{
		Name:  name,
		Table: table,
	}
	if string(c.Table) == "" {
		c.Table = Filter
	}
	return c.Remove()
}

// Add forwarding rule to 'filter' table and corresponding nat rule to 'nat' table
func (c *Chain) Forward(action Action, ip net.IP, port int, proto, destAddr string, destPort int) error {
	daddr := ip.String()
	if ip.IsUnspecified() {
		// iptables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
		// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
		// value" by both iptables and ip6tables.
		daddr = "0/0"
	}
	if output, err := Raw("-t", string(Nat), string(action), c.Name,
		"-p", proto,
		"-d", daddr,
		"--dport", strconv.Itoa(port),
		"!", "-i", c.Bridge,
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(destAddr, strconv.Itoa(destPort))); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "FORWARD", Output: output}
	}

	if output, err := Raw("-t", string(Filter), string(action), c.Name,
		"!", "-i", c.Bridge,
		"-o", c.Bridge,
		"-p", proto,
		"-d", destAddr,
		"--dport", strconv.Itoa(destPort),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "FORWARD", Output: output}
	}

	if output, err := Raw("-t", string(Nat), string(action), "POSTROUTING",
		"-p", proto,
		"-s", destAddr,
		"-d", destAddr,
		"--dport", strconv.Itoa(destPort),
		"-j", "MASQUERADE"); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "FORWARD", Output: output}
	}

	return nil
}

// Add reciprocal ACCEPT rule for two supplied IP addresses.
// Traffic is allowed from ip1 to ip2 and vice-versa
func (c *Chain) Link(action Action, ip1, ip2 net.IP, port int, proto string) error {
	if output, err := Raw("-t", string(Filter), string(action), c.Name,
		"-i", c.Bridge, "-o", c.Bridge,
		"-p", proto,
		"-s", ip1.String(),
		"-d", ip2.String(),
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error iptables forward: %s", output)
	}
	if output, err := Raw("-t", string(Filter), string(action), c.Name,
		"-i", c.Bridge, "-o", c.Bridge,
		"-p", proto,
		"-s", ip2.String(),
		"-d", ip1.String(),
		"--sport", strconv.Itoa(port),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error iptables forward: %s", output)
	}
	return nil
}

// Add linking rule to nat/PREROUTING chain.
func (c *Chain) Prerouting(action Action, args ...string) error {
	a := []string{"-t", string(Nat), string(action), "PREROUTING"}
	if len(args) > 0 {
		a = append(a, args...)
	}
	if output, err := Raw(append(a, "-j", c.Name)...); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "PREROUTING", Output: output}
	}
	return nil
}

// Add linking rule to an OUTPUT chain
func (c *Chain) Output(action Action, args ...string) error {
	a := []string{"-t", string(c.Table), string(action), "OUTPUT"}
	if len(args) > 0 {
		a = append(a, args...)
	}
	if output, err := Raw(append(a, "-j", c.Name)...); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "OUTPUT", Output: output}
	}
	return nil
}

func (c *Chain) Remove() error {
	// Ignore errors - This could mean the chains were never set up
	if c.Table == Nat {
		c.Prerouting(Delete, "-m", "addrtype", "--dst-type", "LOCAL")
		c.Output(Delete, "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8")
		c.Output(Delete, "-m", "addrtype", "--dst-type", "LOCAL") // Created in versions <= 0.1.6

		c.Prerouting(Delete)
		c.Output(Delete)
	}
	Raw("-t", string(c.Table), "-F", c.Name)
	Raw("-t", string(c.Table), "-X", c.Name)
	return nil
}

// Check if a rule exists
func Exists(table Table, chain string, rule ...string) bool {
	if string(table) == "" {
		table = Filter
	}

	// iptables -C, --check option was added in v.1.4.11
	// http://ftp.netfilter.org/pub/iptables/changes-iptables-1.4.11.txt

	// try -C
	// if exit status is 0 then return true, the rule exists
	if _, err := Raw(append([]string{
		"-t", string(table), "-C", chain}, rule...)...); err == nil {
		return true
	}

	// parse "iptables -S" for the rule (this checks rules in a specific chain
	// in a specific table)
	rule_string := strings.Join(rule, " ")
	existingRules, _ := exec.Command("iptables", "-t", string(table), "-S", chain).Output()

	// regex to replace ips in rule
	// because MASQUERADE rule will not be exactly what was passed
	re := regexp.MustCompile(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\/[0-9]{1,2}`)

	return strings.Contains(
		re.ReplaceAllString(string(existingRules), "?"),
		re.ReplaceAllString(rule_string, "?"),
	)
}

// Call 'iptables' system command, passing supplied arguments
func Raw(args ...string) ([]byte, error) {

	if err := initCheck(); err != nil {
		return nil, err
	}
	if supportsXlock {
		args = append([]string{"--wait"}, args...)
	}

	log.Debugf("%s, %v", iptablesPath, args)

	output, err := exec.Command(iptablesPath, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iptables failed: iptables %v: %s (%s)", strings.Join(args, " "), output, err)
	}

	// ignore iptables' message about xtables lock
	if strings.Contains(string(output), "waiting for it to exit") {
		output = []byte("")
	}

	return output, err
}
