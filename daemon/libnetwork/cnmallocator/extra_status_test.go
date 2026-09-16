package cnmallocator

import (
	"context"
	"net/netip"
	"testing"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/moby/moby/v2/daemon/cluster/convert/netextra"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// ipamStatusAppdata is the appdata a client sends to ask for IPAM status,
// as [github.com/moby/moby/v2/daemon/cluster.Cluster] marshals it.
func ipamStatusAppdata(t *testing.T) (typeurl string, appdata []byte) {
	t.Helper()
	msg, err := gogotypes.MarshalAny(&netextra.GetNetworkExtraOptions{WithIPAMStatus: true})
	assert.NilError(t, err)
	return msg.TypeUrl, msg.Value
}

func newOnGetNetworker(t *testing.T) networkallocator.OnGetNetworker {
	t.Helper()
	na, ok := newNetworkAllocator(t).(networkallocator.OnGetNetworker)
	assert.Assert(t, ok, "allocator does not implement OnGetNetworker")
	return na
}

// TestOnGetNetworkUnallocated asserts the [networkallocator.OnGetNetworker]
// contract: "The network may not have been allocated at the time of the call.
// Calling OnGetNetwork with an unallocated network should not be an error."
//
// An unallocated network is not hypothetical. It is the state a failed
// allocation leaves behind - Allocate frees the pools and does not record the
// network - and the allocator only logs the failure before parking the network
// for a later retry. It is also the state of every network in the store that
// the allocator has not restored yet during manager init.
//
// Erroring here does not merely lose one network's status. The hook is called
// per-network over the whole store for a ListNetworks request with no filters,
// which is how the daemon lists Swarm networks, so one unallocated network
// would fail the response for all of them.
func TestOnGetNetworkUnallocated(t *testing.T) {
	na := newOnGetNetworker(t)
	typeurl, appdata := ipamStatusAppdata(t)

	// A network as it looks in the store when allocation failed: the spec is
	// committed, but the allocator never filled in IPAM or the driver state.
	n := &api.Network{
		ID: "unallocatedID",
		Spec: api.NetworkSpec{
			Annotations:  api.Annotations{Name: "unallocated"},
			DriverConfig: &api.Driver{Name: "overlay"},
		},
	}

	assert.NilError(t, na.OnGetNetwork(context.Background(), n, typeurl, appdata))
	// There is no status to report, rather than an empty one: a client cannot
	// tell an absent Status from a manager too old to report any, and the
	// unallocated condition is already visible in the empty IPAM config.
	assert.Check(t, is.Nil(n.Extra))
}

// TestOnGetNetworkAllocated is the positive control for
// [TestOnGetNetworkUnallocated]: returning no status for an unallocated
// network is only correct if an allocated one still gets one.
func TestOnGetNetworkAllocated(t *testing.T) {
	na := newOnGetNetworker(t)
	typeurl, appdata := ipamStatusAppdata(t)

	subnet := netip.MustParsePrefix("10.144.0.0/24")
	n := &api.Network{
		ID: "allocatedID",
		Spec: api.NetworkSpec{
			Annotations:  api.Annotations{Name: "allocated"},
			DriverConfig: &api.Driver{Name: "overlay"},
			IPAM: &api.IPAMOptions{
				Configs: []*api.IPAMConfig{{
					Family: api.IPAMConfig_IPV4,
					Subnet: subnet.String(),
				}},
			},
		},
	}
	assert.NilError(t, na.(networkallocator.NetworkAllocator).Allocate(n))

	assert.NilError(t, na.OnGetNetwork(context.Background(), n, typeurl, appdata))
	assert.Assert(t, n.Extra != nil, "no status reported for an allocated network")

	status, err := netextra.StatusFrom(n.Extra)
	assert.NilError(t, err)
	assert.Assert(t, status != nil)
	st, ok := status.IPAM.Subnets[subnet]
	assert.Assert(t, ok, "no IPAM status for subnet %s, only for %v", subnet, status.IPAM.Subnets)
	// The gateway is allocated out of the subnet, so the counters are already
	// non-trivial before anything attaches.
	assert.Check(t, st.IPsInUse > 0, "IPsInUse is %d for a subnet with a gateway", st.IPsInUse)
	assert.Check(t, st.DynamicIPsAvailable > 0, "no addresses available in a freshly allocated /24")
}

// TestOnGetNetworkWithoutStatusRequested checks that a request which did not
// ask for status does not get any, whether or not the network is allocated.
func TestOnGetNetworkWithoutStatusRequested(t *testing.T) {
	na := newOnGetNetworker(t)

	n := &api.Network{
		ID: "nostatusID",
		Spec: api.NetworkSpec{
			Annotations:  api.Annotations{Name: "nostatus"},
			DriverConfig: &api.Driver{Name: "overlay"},
		},
	}

	assert.NilError(t, na.OnGetNetwork(context.Background(), n, "", nil))
	assert.Check(t, is.Nil(n.Extra))

	assert.NilError(t, na.(networkallocator.NetworkAllocator).Allocate(n))
	assert.NilError(t, na.OnGetNetwork(context.Background(), n, "", nil))
	assert.Check(t, is.Nil(n.Extra))
}
