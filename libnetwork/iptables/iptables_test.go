//go:build linux

package iptables

import (
	"net"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
)

const (
	chainName  = "DOCKEREST"
	bridgeName = "lo"
)

func createNewChain(t *testing.T) (*IPTable, *ChainInfo, *ChainInfo) {
	t.Helper()
	iptable := GetIptable(IPv4)

	natChain, err := iptable.NewChain(chainName, Nat)
	if err != nil {
		t.Fatal(err)
	}

	filterChain, err := iptable.NewChain(chainName, Filter)
	if err != nil {
		t.Fatal(err)
	}

	return iptable, natChain, filterChain
}

func TestNewChain(t *testing.T) {
	createNewChain(t)
}

func TestLink(t *testing.T) {
	iptable, _, filterChain := createNewChain(t)
	ip1 := netip.MustParseAddr("192.168.1.1")
	ip2 := netip.MustParseAddr("192.168.1.2")
	port := 1234
	proto := "tcp"

	err := filterChain.Link(Append, ip1, ip2, port, proto, bridgeName)
	if err != nil {
		t.Fatal(err)
	}

	rule1 := []string{
		"-i", bridgeName,
		"-o", bridgeName,
		"-p", proto,
		"-s", ip1.String(),
		"-d", ip2.String(),
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT",
	}

	if !iptable.Exists(filterChain.Table, filterChain.Name, rule1...) {
		t.Fatal("rule1 does not exist")
	}

	rule2 := []string{
		"-i", bridgeName,
		"-o", bridgeName,
		"-p", proto,
		"-s", ip2.String(),
		"-d", ip1.String(),
		"--sport", strconv.Itoa(port),
		"-j", "ACCEPT",
	}

	if !iptable.Exists(filterChain.Table, filterChain.Name, rule2...) {
		t.Fatal("rule2 does not exist")
	}
}

func TestPrerouting(t *testing.T) {
	iptable, natChain, _ := createNewChain(t)

	args := []string{"-i", "lo", "-d", "192.168.1.1"}
	err := natChain.Prerouting(Insert, args...)
	if err != nil {
		t.Fatal(err)
	}

	if !iptable.Exists(natChain.Table, "PREROUTING", args...) {
		t.Fatal("rule does not exist")
	}

	delRule := append([]string{"-D", "PREROUTING", "-t", string(Nat)}, args...)
	if _, err = iptable.Raw(delRule...); err != nil {
		t.Fatal(err)
	}
}

func TestOutput(t *testing.T) {
	iptable, natChain, _ := createNewChain(t)

	args := []string{"-o", "lo", "-d", "192.168.1.1"}
	err := natChain.Output(Insert, args...)
	if err != nil {
		t.Fatal(err)
	}

	if !iptable.Exists(natChain.Table, "OUTPUT", args...) {
		t.Fatal("rule does not exist")
	}

	delRule := append([]string{
		"-D", "OUTPUT", "-t",
		string(natChain.Table),
	}, args...)
	if _, err = iptable.Raw(delRule...); err != nil {
		t.Fatal(err)
	}
}

// Runs 10 concurrent rule additions. This will fail if iptables
// is actually invoked simultaneously without --wait.
func TestConcurrencyWithWait(t *testing.T) {
	_, natChain, _ := createNewChain(t)

	ip := net.ParseIP("192.168.1.1")
	port := 1234
	dstAddr := "172.17.0.1"
	dstPort := 4321
	proto := "tcp"

	group := new(errgroup.Group)
	for i := 0; i < 10; i++ {
		group.Go(func() error {
			return addSomeRules(natChain, ip, port, proto, dstAddr, dstPort)
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
}

// addSomeRules adds arbitrary iptable rules. RunConcurrencyTest previously used
// iptables.Forward to create rules, that function has been removed. To preserve
// the test, this function creates similar rules.
func addSomeRules(c *ChainInfo, ip net.IP, port int, proto, destAddr string, destPort int) error {
	iptable := GetIptable(c.IPVersion)
	daddr := ip.String()

	args := []string{
		"-p", proto,
		"-d", daddr,
		"--dport", strconv.Itoa(port),
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(destAddr, strconv.Itoa(destPort)),
	}
	if err := iptable.ProgramRule(Nat, c.Name, Append, args); err != nil {
		return err
	}

	args = []string{
		"!", "-i", "lo",
		"-o", "lo",
		"-p", proto,
		"-d", destAddr,
		"--dport", strconv.Itoa(destPort),
		"-j", "ACCEPT",
	}
	if err := iptable.ProgramRule(Filter, c.Name, Append, args); err != nil {
		return err
	}

	args = []string{
		"-p", proto,
		"-s", destAddr,
		"-d", destAddr,
		"--dport", strconv.Itoa(destPort),
		"-j", "MASQUERADE",
	}
	if err := iptable.ProgramRule(Nat, "POSTROUTING", Append, args); err != nil {
		return err
	}

	return nil
}

func TestCleanup(t *testing.T) {
	iptable, _, filterChain := createNewChain(t)

	filterChain.Remove()

	err := iptable.RemoveExistingChain(chainName, Nat)
	if err != nil {
		t.Fatal(err)
	}

	rules, err := exec.Command("iptables-save").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rules), chainName) {
		t.Fatalf("Removing chain failed. %s found in iptables-save", chainName)
	}
}

func TestExistsRaw(t *testing.T) {
	const testChain1 = "ABCD"
	const testChain2 = "EFGH"

	iptable := GetIptable(IPv4)

	_, err := iptable.NewChain(testChain1, Filter)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		iptable.RemoveExistingChain(testChain1, Filter)
	}()

	_, err = iptable.NewChain(testChain2, Filter)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		iptable.RemoveExistingChain(testChain2, Filter)
	}()

	// Test detection over full and truncated rule string
	input := []struct{ rule []string }{
		{[]string{"-s", "172.8.9.9/32", "-j", "ACCEPT"}},
		{[]string{"-d", "172.8.9.0/24", "-j", "DROP"}},
		{[]string{"-s", "172.0.3.0/24", "-d", "172.17.0.0/24", "-p", "tcp", "-m", "tcp", "--dport", "80", "-j", testChain2}},
		{[]string{"-j", "RETURN"}},
	}

	for i, r := range input {
		ruleAdd := append([]string{"-t", string(Filter), "-A", testChain1}, r.rule...)
		err = iptable.RawCombinedOutput(ruleAdd...)
		if err != nil {
			t.Fatalf("i=%d, err: %v", i, err)
		}
		if !iptable.exists(true, Filter, testChain1, r.rule...) {
			t.Fatalf("Failed to detect rule. i=%d", i)
		}
		// Truncate the rule
		trg := r.rule[len(r.rule)-1]
		trg = trg[:len(trg)-2]
		r.rule[len(r.rule)-1] = trg
		if iptable.exists(true, Filter, testChain1, r.rule...) {
			t.Fatalf("Invalid detection. i=%d", i)
		}
	}
}

func TestRule(t *testing.T) {
	_ = firewalldInit()
	if res, _ := UsingFirewalld(); res {
		t.Skip("firewalld in host netns cannot create rules in the test's netns")
	}
	defer netnsutils.SetupTestOSContext(t)()

	assert.NilError(t, exec.Command("iptables", "-N", "TESTCHAIN").Run())

	rule := Rule{IPVer: IPv4, Table: Filter, Chain: "TESTCHAIN", Args: []string{"-j", "RETURN"}}
	assert.NilError(t, rule.Insert())
	assert.Equal(t, rule.Exists(), true)
	assert.Equal(t, mustDumpChain(t, Filter, "TESTCHAIN"), `-N TESTCHAIN
-A TESTCHAIN -j RETURN
`)

	// Test that Insert is idempotent
	assert.NilError(t, rule.Insert())
	assert.Equal(t, mustDumpChain(t, Filter, "TESTCHAIN"), `-N TESTCHAIN
-A TESTCHAIN -j RETURN
`)

	rule = Rule{IPVer: IPv4, Table: Filter, Chain: "TESTCHAIN", Args: []string{"-j", "ACCEPT"}}
	assert.NilError(t, rule.Append())
	assert.Equal(t, rule.Exists(), true)
	assert.Equal(t, mustDumpChain(t, Filter, "TESTCHAIN"), `-N TESTCHAIN
-A TESTCHAIN -j RETURN
-A TESTCHAIN -j ACCEPT
`)

	// Test that Append is idempotent
	assert.NilError(t, rule.Append())
	assert.Equal(t, mustDumpChain(t, Filter, "TESTCHAIN"), `-N TESTCHAIN
-A TESTCHAIN -j RETURN
-A TESTCHAIN -j ACCEPT
`)

	assert.NilError(t, rule.Delete())
	assert.Equal(t, rule.Exists(), false)
	assert.Equal(t, mustDumpChain(t, Filter, "TESTCHAIN"), `-N TESTCHAIN
-A TESTCHAIN -j RETURN
`)

	// Test that Delete is idempotent
	assert.NilError(t, rule.Delete())
}

func mustDumpChain(t *testing.T, table Table, chain string) string {
	t.Helper()
	out, err := exec.Command("iptables", "-t", string(table), "-S", chain).CombinedOutput()
	assert.NilError(t, err, "output:\n%s", out)
	return string(out)
}
