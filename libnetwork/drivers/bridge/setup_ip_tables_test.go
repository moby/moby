package bridge

import (
	"net"
	"testing"

	"github.com/docker/docker/pkg/iptables"
	"github.com/docker/libnetwork"
)

const (
	iptablesTestBridgeIP = "192.168.42.1"
)

func TestProgramIPTable(t *testing.T) {
	// Create a test bridge with a basic bridge configuration (name + IPv4).
	defer libnetwork.SetupTestNetNS(t)()
	createTestBridge(getBasicTestConfig(), t)

	// Store various iptables chain rules we care for.
	rules := []struct {
		ruleArgs []string
		descr    string
	}{{[]string{"FORWARD", "-d", "127.1.2.3", "-i", "lo", "-o", "lo", "-j", "DROP"}, "Test Loopback"},
		{[]string{"POSTROUTING", "-t", "nat", "-s", iptablesTestBridgeIP, "!", "-o", DefaultBridgeName, "-j", "MASQUERADE"}, "NAT Test"},
		{[]string{"FORWARD", "-i", DefaultBridgeName, "!", "-o", DefaultBridgeName, "-j", "ACCEPT"}, "Test ACCEPT NON_ICC OUTGOING"},
		{[]string{"FORWARD", "-o", DefaultBridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}, "Test ACCEPT INCOMING"},
		{[]string{"FORWARD", "-i", DefaultBridgeName, "-o", DefaultBridgeName, "-j", "ACCEPT"}, "Test enable ICC"},
		{[]string{"FORWARD", "-i", DefaultBridgeName, "-o", DefaultBridgeName, "-j", "DROP"}, "Test disable ICC"},
	}

	// Assert the chain rules' insertion and removal.
	for _, c := range rules {
		assertIPTableChainProgramming(c.ruleArgs, c.descr, t)
	}
}

func TestSetupIPTables(t *testing.T) {
	// Create a test bridge with a basic bridge configuration (name + IPv4).
	defer libnetwork.SetupTestNetNS(t)()
	br := getBasicTestConfig()
	createTestBridge(br, t)

	// Modify iptables params in base configuration and apply them.
	br.Config.EnableIPTables = true
	assertBridgeConfig(br, t)

	br.Config.EnableIPMasquerade = true
	assertBridgeConfig(br, t)

	br.Config.EnableICC = true
	assertBridgeConfig(br, t)

	br.Config.EnableIPMasquerade = false
	assertBridgeConfig(br, t)
}

func getBasicTestConfig() *bridgeInterface {
	return &bridgeInterface{
		Config: &Configuration{
			BridgeName:  DefaultBridgeName,
			AddressIPv4: &net.IPNet{IP: net.ParseIP(iptablesTestBridgeIP), Mask: net.CIDRMask(16, 32)},
		},
	}
}

func createTestBridge(br *bridgeInterface, t *testing.T) {
	if err := setupDevice(br); err != nil {
		t.Fatalf("Failed to create the testing Bridge: %s", err.Error())
	}
	if err := setupBridgeIPv4(br); err != nil {
		t.Fatalf("Failed to bring up the testing Bridge: %s", err.Error())
	}
}

// Assert base function which pushes iptables chain rules on insertion and removal.
func assertIPTableChainProgramming(args []string, descr string, t *testing.T) {
	// Add
	if err := programChainRule(args, descr, true); err != nil {
		t.Fatalf("Failed to program iptable rule %s: %s", descr, err.Error())
	}
	if iptables.Exists(args...) == false {
		t.Fatalf("Failed to effectively program iptable rule: %s", descr)
	}

	// Remove
	if err := programChainRule(args, descr, false); err != nil {
		t.Fatalf("Failed to remove iptable rule %s: %s", descr, err.Error())
	}
	if iptables.Exists(args...) == true {
		t.Fatalf("Failed to effectively remove iptable rule: %s", descr)
	}
}

// Assert function which pushes chains based on bridge config parameters.
func assertBridgeConfig(br *bridgeInterface, t *testing.T) {
	// Attempt programming of ip tables.
	err := setupIPTables(br)
	if err != nil {
		t.Fatalf("%v", err)
	}
}
