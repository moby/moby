package swarm

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/engine-api/types/swarm"
)

func TestNodeAddrOptionSetHostAndPort(t *testing.T) {
	opt := NewNodeAddrOption("old:123")
	addr := "newhost:5555"
	assert.NilError(t, opt.Set(addr))
	assert.Equal(t, opt.Value(), addr)
}

func TestNodeAddrOptionSetHostOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("newhost"))
	assert.Equal(t, opt.Value(), "newhost:2377")
}

func TestNodeAddrOptionSetHostOnlyIPv6(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("::1"))
	assert.Equal(t, opt.Value(), "[::1]:2377")
}

func TestNodeAddrOptionSetPortOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set(":4545"))
	assert.Equal(t, opt.Value(), "0.0.0.0:4545")
}

func TestNodeAddrOptionSetInvalidFormat(t *testing.T) {
	opt := NewListenAddrOption()
	assert.Error(t, opt.Set("http://localhost:4545"), "Invalid")
}

func TestAutoAcceptOptionSetWorker(t *testing.T) {
	opt := NewAutoAcceptOption()
	assert.NilError(t, opt.Set("worker"))
	assert.Equal(t, opt.values[WORKER], true)
}

func TestAutoAcceptOptionSetManager(t *testing.T) {
	opt := NewAutoAcceptOption()
	assert.NilError(t, opt.Set("manager"))
	assert.Equal(t, opt.values[MANAGER], true)
}

func TestAutoAcceptOptionSetInvalid(t *testing.T) {
	opt := NewAutoAcceptOption()
	assert.Error(t, opt.Set("bogus"), "must be one of")
}

func TestAutoAcceptOptionSetNone(t *testing.T) {
	opt := NewAutoAcceptOption()
	assert.NilError(t, opt.Set("none"))
	assert.Equal(t, opt.values[MANAGER], false)
	assert.Equal(t, opt.values[WORKER], false)
}

func TestAutoAcceptOptionSetConflict(t *testing.T) {
	opt := NewAutoAcceptOption()
	assert.NilError(t, opt.Set("manager"))
	assert.Error(t, opt.Set("none"), "value NONE is incompatible with MANAGER")

	opt = NewAutoAcceptOption()
	assert.NilError(t, opt.Set("none"))
	assert.Error(t, opt.Set("worker"), "value NONE is incompatible with WORKER")
}

func TestAutoAcceptOptionPoliciesDefault(t *testing.T) {
	opt := NewAutoAcceptOption()
	secret := "thesecret"

	policies := opt.Policies(&secret)
	assert.Equal(t, len(policies), 2)
	assert.Equal(t, policies[0], swarm.Policy{
		Role:       WORKER,
		Autoaccept: true,
		Secret:     &secret,
	})
	assert.Equal(t, policies[1], swarm.Policy{
		Role:       MANAGER,
		Autoaccept: false,
		Secret:     &secret,
	})
}

func TestAutoAcceptOptionPoliciesWithManager(t *testing.T) {
	opt := NewAutoAcceptOption()
	secret := "thesecret"

	assert.NilError(t, opt.Set("manager"))

	policies := opt.Policies(&secret)
	assert.Equal(t, len(policies), 2)
	assert.Equal(t, policies[0], swarm.Policy{
		Role:       WORKER,
		Autoaccept: false,
		Secret:     &secret,
	})
	assert.Equal(t, policies[1], swarm.Policy{
		Role:       MANAGER,
		Autoaccept: true,
		Secret:     &secret,
	})
}

func TestAutoAcceptOptionString(t *testing.T) {
	opt := NewAutoAcceptOption()
	assert.NilError(t, opt.Set("manager"))
	assert.NilError(t, opt.Set("worker"))

	repr := opt.String()
	assert.Contains(t, repr, "worker=true")
	assert.Contains(t, repr, "manager=true")
}
