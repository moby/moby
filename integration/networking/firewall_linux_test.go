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

	exp := "iptables"
	if networking.FirewalldRunning() {
		exp = "iptables+firewalld"
	}
	info, err := c.Info(ctx)
	assert.NilError(t, err)
	t.Log("FirewallBackend:", info.FirewallBackend)
	assert.Check(t, is.Equal(info.FirewallBackend, exp))
}
