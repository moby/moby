package networking

import (
	"os/exec"
	"regexp"
	"testing"
)

// Find the policy in, for example "Chain FORWARD (policy ACCEPT)".
var rePolicy = regexp.MustCompile("policy ([A-Z]+)")

// SetFilterForwardPolicies sets the default policy for the FORWARD chain in
// the filter tables for both IPv4 and IPv6. The original policy is restored
// using t.Cleanup().
//
// There's only one filter-FORWARD policy, so this won't behave well if used by
// tests running in parallel in a single network namespace that expect different
// behaviour.
func SetFilterForwardPolicies(t *testing.T, policy string) {
	t.Helper()

	for _, iptablesCmd := range []string{"iptables", "ip6tables"} {
		cmd := exec.Command(iptablesCmd, "-L", "FORWARD")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("Failed to get %s FORWARD policy: %v", iptablesCmd, err)
		}
		opMatch := rePolicy.FindSubmatch(out)
		if len(opMatch) != 2 {
			t.Fatalf("Failed to find %s FORWARD policy in: %s", iptablesCmd, out)
		}
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
