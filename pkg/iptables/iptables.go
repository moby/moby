package iptables

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Action string

const (
	Add    Action = "-A"
	Delete Action = "-D"
	InternalNetwork string = "10.0.0.0/16"
)

var (
	ErrIptablesNotFound = errors.New("Iptables not found")
	nat                 = []string{"-t", "nat"}
)

type Chain struct {
	Name   string
	Bridge string
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

	if err := chain.Prerouting(Add, "-m", "addrtype", "--dst-type", "LOCAL"); err != nil {
		return nil, fmt.Errorf("Failed to inject docker in PREROUTING chain: %s", err)
	}
	if err := chain.Output(Add, "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8"); err != nil {
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
	if output, err := Raw("-t", "nat", fmt.Sprint(action), c.Name,
		"-p", proto,
		"-d", daddr,
		"--dport", strconv.Itoa(port),
		"!", "-i", c.Bridge,
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(dest_addr, strconv.Itoa(dest_port))); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error iptables forward: %s", output)
	}

	fAction := action
	if fAction == Add {
		fAction = "-I"
	}
	if output, err := Raw(string(fAction), "FORWARD",
		"!", "-i", c.Bridge,
		"-o", c.Bridge,
		"-p", proto,
		"-d", dest_addr,
		"--dport", strconv.Itoa(dest_port),
		"-j", "ACCEPT"); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error iptables forward: %s", output)
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
		return fmt.Errorf("Error iptables prerouting: %s", output)
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
		return fmt.Errorf("Error iptables output: %s", output)
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

func CreateNetworkMetricRules(ip string) error {

	if ExistsNetworkMetricRule(ip) == true {
		return fmt.Errorf("Error when creating metrics rules for %s", ip)
	}

	if input, err := Raw("-I", "FORWARD", "1", "-o", "docker0", "-d", ip, "!", "-s", InternalNetwork); err != nil {
		return err
	} else if len(input) != 0 {
		return fmt.Errorf("Error when creating metrics input rule: %s", input)
	}

	if output, err := Raw("-I", "FORWARD", "1", "-i", "docker0", "!", "-o", "docker0", "-s", ip, "!", "-d", InternalNetwork); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error when creating metrics output rule: %s", output)
	}

	return nil
}

func DeleteNetworkMetricRules(ip string) error {

	if ExistsNetworkMetricRule(ip) == false {
		return fmt.Errorf("Error when deleting metrics rules for %s", ip)
	}

	if input, err := Raw("-D", "FORWARD", "-o", "docker0", "-d", ip, "!", "-s", InternalNetwork); err != nil {
		return err
	} else if len(input) != 0 {
		return fmt.Errorf("Error when deleting metrics input rule: %s", input)
	}

	if output, err := Raw("-D", "FORWARD", "-i", "docker0", "!", "-o", "docker0", "-s", ip, "!", "-d", InternalNetwork); err != nil {
		return err
	} else if len(output) != 0 {
		return fmt.Errorf("Error when deleting metrics output rule: %s", output)
	}

	return nil
}

func ExistsNetworkMetricRule(ip string) bool {

	input := Exists("FORWARD", "-o", "docker0", "-d", ip, "!", "-s", InternalNetwork)
	output := Exists("FORWARD", "-i", "docker0", "!", "-o", "docker0", "-s", ip, "!", "-d", InternalNetwork)
	fmt.Println("EXISTS INPUT:", input)
	fmt.Println("EXISTS OUTPUT:", output)
	fmt.Println("EXISTS:", ((input == output) && (input == true)))
	return ((input == output) && (input == true))
}

// Check if an existing rule exists
func Exists(args ...string) bool {
	if _, err := Raw(append([]string{"-C"}, args...)...); err != nil {
		return false
	}
	return true
}

func Raw(args ...string) ([]byte, error) {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return nil, ErrIptablesNotFound
	}
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("[DEBUG] [iptables]: %s, %v\n", path, args)
	}
	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iptables failed: iptables %v: %s (%s)", strings.Join(args, " "), output, err)
	}
	return output, err
}
