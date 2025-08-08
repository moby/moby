package networking

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/moby/moby/v2/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
)

// Find the policy in, for example "Chain FORWARD (policy ACCEPT)".
var rePolicy = regexp.MustCompile("policy ([A-Za-z]+)")

// SetFilterForwardPolicies sets the default policy for the FORWARD chain in
// the iptables filter tables for both IPv4 and IPv6. The original policy is
// restored using t.Cleanup().
//
// There's only one filter-FORWARD policy, so this won't behave well if used by
// tests running in parallel in a single network namespace that expect different
// behaviour.
func SetFilterForwardPolicies(t *testing.T, policy string) {
	t.Helper()
	for _, iptablesCmd := range []string{"iptables", "ip6tables"} {
		out, err := exec.Command(iptablesCmd, "-L", "FORWARD").Output()
		assert.NilError(t, err, "failed to get %s policy", iptablesCmd)
		opMatch := rePolicy.FindSubmatch(out)
		assert.Assert(t, is.Len(opMatch, 2), "searching for policy: %w", err)
		origPolicy := string(opMatch[1])
		if origPolicy == policy {
			continue
		}
		if err := exec.Command(iptablesCmd, "-P", "FORWARD", policy).Run(); err != nil {
			t.Fatalf("Failed to set %s FORWARD policy: %v", iptablesCmd, err)
		}
		t.Cleanup(func() {
			if err := exec.Command(iptablesCmd, "-P", "FORWARD", origPolicy).Run(); err != nil {
				t.Logf("Failed to restore %s FORWARD policy: %v", iptablesCmd, err)
			}
		})
	}
}

// FirewalldRunning returns true if "firewall-cmd --state" reports "running".
func FirewalldRunning() bool {
	state, err := exec.Command("firewall-cmd", "--state").CombinedOutput()
	return err == nil && strings.TrimSpace(string(state)) == "running"
}

// FirewalldReload reloads firewalld and waits for the daemon to re-create its rules.
// It's a no-op if firewalld is not running, and the test fails if the reload does
// not complete.
func FirewalldReload(t *testing.T, d *daemon.Daemon) {
	t.Helper()
	if !FirewalldRunning() {
		return
	}
	lastReload := d.FirewallReloadedAt(t)
	res := icmd.RunCommand("firewall-cmd", "--reload")
	assert.NilError(t, res.Error)

	poll.WaitOn(t, func(_ poll.LogT) poll.Result {
		latestReload := d.FirewallReloadedAt(t)
		if latestReload != "" && latestReload != lastReload {
			t.Log("Firewalld reload completed at", latestReload)
			return poll.Success()
		}
		return poll.Continue("firewalld reload not complete")
	})
}
