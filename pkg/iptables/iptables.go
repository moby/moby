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

const (
	Append Action = "-A"
	Delete Action = "-D"
	Insert Action = "-I"
)

var (
	nat                 = []string{"-t", "nat"}
	supportsXlock       = false
	ErrIptablesNotFound = errors.New("Iptables not found")
)

type Chain struct {
	Name   string
	Bridge string
}

type ChainError struct {
	Chain  string
	Output []byte
}

func (e *ChainError) Error() string {
	return fmt.Sprintf("Error iptables %s: %s", e.Chain, string(e.Output))
}

func init() {
	supportsXlock = exec.Command("iptables", "--wait", "-L", "-n").Run() == nil
}

func NewChain(name, bridge string) (*Chain, error) {
	if output, err := Raw("-t", "nat", "-N", name); err != nil {
		return nil, err
	} else if len(output) != 0 {
		return nil, fmt.Errorf("Error creating new iptables chain: %s", output)
	}
	chain := &Chain{
		Name:   name,
		Bridge: bridge,
	}

	if err := chain.Prerouting(Append, "-m", "addrtype", "--dst-type", "LOCAL"); err != nil {
		return nil, fmt.Errorf("Failed to inject docker in PREROUTING chain: %s", err)
	}
	if err := chain.Output(Append, "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8"); err != nil {
		return nil, fmt.Errorf("Failed to inject docker in OUTPUT chain: %s", err)
	}
	return chain, nil
}

func RemoveExistingChain(name string) error {
	chain := &Chain{
		Name: name,
	}
	return chain.Remove()
}

func (c *Chain) Forward(action Action, ip net.IP, port int, proto, dest_addr string, dest_port int) error {
	daddr := ip.String()
	if ip.IsUnspecified() {
		// iptables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
		// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
		// value" by both iptables and ip6tables.
		daddr = "0/0"
	}
	if output, err := Raw("-t", "nat", string(action), c.Name,
		"-p", proto,
		"-d", daddr,
		"--dport", strconv.Itoa(port),
		"!", "-i", c.Bridge,
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(dest_addr, strconv.Itoa(dest_port))); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "FORWARD", Output: output}
	}

	if action != Delete {
		if err := c.createForwardChain(); err != nil {
			return err
		}
	}

	if output, err := Raw(string(action), c.Name,
		"!", "-i", c.Bridge,
		"-o", c.Bridge,
		"-p", proto,
		"-d", dest_addr,
		"--dport", strconv.Itoa(dest_port),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return &ChainError{Chain: "FORWARD", Output: output}
	}

	return nil
}

func (c *Chain) Link(action Action, ip1, ip2 net.IP, port int, proto string) error {
	if action != Delete {
		if err := c.createForwardChain(); err != nil {
			return err
		}
	}
	if output, err := Raw(string(action), c.Name,
		"-i", c.Bridge, "-o", c.Bridge,
		"-p", proto,
		"-s", ip1.String(),
		"--dport", strconv.Itoa(port),
		"-d", ip2.String(),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error toggle iptables forward: %s", output)
	}

	if output, err := Raw(string(action), c.Name,
		"-i", c.Bridge, "-o", c.Bridge,
		"-p", proto,
		"-s", ip2.String(),
		"--dport", strconv.Itoa(port),
		"-d", ip1.String(),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error toggle iptables forward: %s", output)
	}

	return nil
}

func (c *Chain) Prerouting(action Action, args ...string) error {
	a := append(nat, fmt.Sprint(action), "PREROUTING")
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

func (c *Chain) Output(action Action, args ...string) error {
	a := append(nat, fmt.Sprint(action), "OUTPUT")
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
	c.Prerouting(Delete, "-m", "addrtype", "--dst-type", "LOCAL")
	c.Output(Delete, "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8")
	c.Output(Delete, "-m", "addrtype", "--dst-type", "LOCAL") // Created in versions <= 0.1.6

	c.Prerouting(Delete)
	c.Output(Delete)

	Raw("-t", "nat", "-F", c.Name)
	Raw("-t", "nat", "-X", c.Name)

	return nil
}

// Check if an existing rule exists
func Exists(args ...string) bool {
	// iptables -C, --check option was added in v.1.4.11
	// http://ftp.netfilter.org/pub/iptables/changes-iptables-1.4.11.txt

	// try -C
	// if exit status is 0 then return true, the rule exists
	if _, err := Raw(append([]string{"-C"}, args...)...); err == nil {
		return true
	}

	// parse iptables-save for the rule
	rule := strings.Replace(strings.Join(args, " "), "-t nat ", "", -1)
	existingRules, _ := exec.Command("iptables-save").Output()

	// regex to replace ips in rule
	// because MASQUERADE rule will not be exactly what was passed
	re := regexp.MustCompile(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\/[0-9]{1,2}`)

	return strings.Contains(
		re.ReplaceAllString(string(existingRules), "?"),
		re.ReplaceAllString(rule, "?"),
	)
}

func Raw(args ...string) ([]byte, error) {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return nil, ErrIptablesNotFound
	}

	if supportsXlock {
		args = append([]string{"--wait"}, args...)
	}

	log.Debugf("%s, %v", path, args)

	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iptables failed: iptables %v: %s (%s)", strings.Join(args, " "), output, err)
	}

	// ignore iptables' message about xtables lock
	if strings.Contains(string(output), "waiting for it to exit") {
		output = []byte("")
	}

	return output, err
}

func (c *Chain) createForwardChain() error {
	// Add chain if doesn't exist
	if _, err := Raw("-n", "-L", c.Name); err != nil {
		output, err := Raw("-N", c.Name)
		if err != nil {
			return err
		} else if len(output) != 0 {
			return fmt.Errorf("Error iptables forward: %s", output)
		}
	}
	// Add linking rule if it doesn't exist
	if !Exists("FORWARD",
		"-o", c.Bridge,
		"-j", c.Name) {
		if output2, err := Raw(string(Insert), "FORWARD",
			"-o", c.Bridge,
			"-j", c.Name); err != nil {
			return err
		} else if len(output2) != 0 {
			return fmt.Errorf("Error iptables forward: %s", output2)
		}
	}
	return nil
}
