package osl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/ipvs"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestApplyOSTweaks checks that ApplyOSTweaks sets the load-balancer sysctls in
// the sandbox's network namespace, and leaves the caller's namespace alone, for
// both the iptables (IPVS) and nftables load balancers.
func TestApplyOSTweaks(t *testing.T) {
	for _, backend := range []struct {
		name    string
		setup   func(t *testing.T)
		initial map[string]string
		want    map[string]string
	}{
		{
			name: "iptables",
			setup: func(t *testing.T) {
				// Ensure the IPVS sysctls exist before setting the sentinel values.
				h, err := ipvs.New("")
				assert.NilError(t, err)
				t.Cleanup(h.Close)
			},
			initial: map[string]string{
				"/proc/sys/net/ipv4/vs/conn_reuse_mode":           "2",
				"/proc/sys/net/ipv4/vs/expire_nodest_conn":        "0",
				"/proc/sys/net/ipv4/vs/expire_quiescent_template": "0",
			},
			want: map[string]string{
				"/proc/sys/net/ipv4/vs/conn_reuse_mode":           "0",
				"/proc/sys/net/ipv4/vs/expire_nodest_conn":        "1",
				"/proc/sys/net/ipv4/vs/expire_quiescent_template": "1",
			},
		},
		{
			name: "nftables",
			setup: func(t *testing.T) {
				if err := nftables.Enable(); err != nil {
					t.Skipf("nftables not available: %v", err)
				}
				t.Cleanup(nftables.Disable)
			},
			initial: map[string]string{"/proc/sys/net/ipv4/ip_forward": "0"},
			want:    map[string]string{"/proc/sys/net/ipv4/ip_forward": "1"},
		},
	} {
		for _, tc := range []struct {
			name  string
			types []SandboxType
		}{
			{name: "load balancer", types: []SandboxType{SandboxTypeLoadBalancer}},
			{name: "ingress", types: []SandboxType{SandboxTypeIngress}},
			{name: "both", types: []SandboxType{SandboxTypeLoadBalancer, SandboxTypeIngress}},
			{name: "ordinary sandbox"},
		} {
			t.Run(backend.name+"/"+tc.name, func(t *testing.T) {
				// Use a disposable namespace as the caller's "host", so the test
				// never changes the real host's configuration.
				defer netnsutils.SetupTestOSContext(t)()
				backend.setup(t)

				n, err := NewSandbox(filepath.Join(t.TempDir(), "sandbox"), true, false)
				assert.NilError(t, err)
				defer destroyTest(t, n)

				want := backend.want
				if len(tc.types) == 0 {
					want = backend.initial
				}

				setInitial := func() {
					for path, value := range backend.initial {
						assert.Check(t, is.Nil(os.WriteFile(path, []byte(value), 0o644)), "setting %s", path)
					}
				}
				check := func(values map[string]string, scope string) {
					for path, value := range values {
						got, err := os.ReadFile(path)
						assert.Check(t, is.Nil(err), "reading %s %s", scope, path)
						assert.Check(t, is.Equal(strings.TrimSpace(string(got)), value), "%s %s", scope, path)
					}
				}

				setInitial()
				assert.NilError(t, n.InvokeFunc(setInitial))

				assert.NilError(t, n.ApplyOSTweaks(tc.types))

				check(backend.initial, "host")
				assert.NilError(t, n.InvokeFunc(func() { check(want, "sandbox") }))
			})
		}
	}
}

func TestApplyOSTweaksMissingNamespace(t *testing.T) {
	n := &Namespace{path: filepath.Join(t.TempDir(), "missing")}
	assert.ErrorContains(t, n.ApplyOSTweaks([]SandboxType{SandboxTypeLoadBalancer}), "failed get network namespace")
	// A sandbox without a load balancer needs no namespace switch.
	assert.NilError(t, n.ApplyOSTweaks(nil))
}
