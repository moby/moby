package networking

import (
	"testing"

	"github.com/docker/docker/internal/testutils/networking"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoFirewallBackend(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	expDriver := "iptables"
	if !testEnv.IsRootless() && networking.FirewalldRunning() {
		expDriver = "iptables+firewalld"
	}
	info, err := c.Info(ctx)
	assert.NilError(t, err)
	assert.Assert(t, info.FirewallBackend != nil, "expected firewall backend in info response")
	t.Log("FirewallBackend: Driver:", info.FirewallBackend.Driver)
	for _, kv := range info.FirewallBackend.Info {
		t.Logf("FirewallBackend: %s: %s", kv[0], kv[1])
	}
	assert.Check(t, is.Equal(info.FirewallBackend.Driver, expDriver))
}
