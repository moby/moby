package defaultipam

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/types"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestKeyString(t *testing.T) {
	k := &PoolID{AddressSpace: "default", SubnetKey: SubnetKey{Subnet: netip.MustParsePrefix("172.27.0.0/16")}}
	expected := "default/172.27.0.0/16"
	if expected != k.String() {
		t.Fatalf("Unexpected key string: %s", k.String())
	}

	k2, err := PoolIDFromString(expected)
	if err != nil {
		t.Fatal(err)
	}
	if k2.AddressSpace != k.AddressSpace || k2.Subnet != k.Subnet {
		t.Fatalf("SubnetKey.FromString() failed. Expected %v. Got %v", k, k2)
	}

	expected = fmt.Sprintf("%s/%s", expected, "172.27.3.0/24")
	k.ChildSubnet = netip.MustParsePrefix("172.27.3.0/24")
	if expected != k.String() {
		t.Fatalf("Unexpected key string: %s", k.String())
	}

	k2, err = PoolIDFromString(expected)
	if err != nil {
		t.Fatal(err)
	}
	if k2.AddressSpace != k.AddressSpace || k2.Subnet != k.Subnet || k2.ChildSubnet != k.ChildSubnet {
		t.Fatalf("SubnetKey.FromString() failed. Expected %v. Got %v", k, k2)
	}
}

func TestAddSubnets(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}

	alloc1, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8"})
	if err != nil {
		t.Fatal("Unexpected failure in adding subnet")
	}

	alloc2, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: globalAddressSpace, Pool: "10.0.0.0/8"})
	if err != nil {
		t.Fatalf("Unexpected failure in adding overlapping subnets to different address spaces: %v", err)
	}

	if alloc1.PoolID == alloc2.PoolID {
		t.Fatal("returned same pool id for same subnets in different namespaces")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: globalAddressSpace, Pool: "10.0.0.0/8"})
	if err == nil {
		t.Fatalf("Expected failure requesting existing subnet")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: globalAddressSpace, Pool: "10.128.0.0/9"})
	if err == nil {
		t.Fatal("Expected failure on adding overlapping base subnet")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: globalAddressSpace, Pool: "10.0.0.0/8", SubPool: "10.128.0.0/9"})
	if err != nil {
		t.Fatalf("Unexpected failure on adding sub pool: %v", err)
	}
	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: globalAddressSpace, Pool: "10.0.0.0/8", SubPool: "10.128.0.0/9"})
	if err == nil {
		t.Fatalf("Expected failure on adding overlapping sub pool")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.20.2.0/24"})
	if err == nil {
		t.Fatal("Failed to detect overlapping subnets")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.128.0.0/9"})
	if err == nil {
		t.Fatal("Failed to detect overlapping subnets")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "1003:1:2:3:4:5:6::/112"})
	if err != nil {
		t.Fatalf("Failed to add v6 subnet: %s", err.Error())
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "1003:1:2:3::/64"})
	if err == nil {
		t.Fatal("Failed to detect overlapping v6 subnet")
	}
}

// TestDoublePoolRelease tests that releasing a pool which has already
// been released raises an error.
func TestDoublePoolRelease(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc1, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8"})
	assert.NilError(t, err)

	err = a.ReleasePool(alloc1.PoolID)
	assert.NilError(t, err)

	err = a.ReleasePool(alloc1.PoolID)
	assert.Check(t, is.ErrorContains(err, ""))
}

func TestAddReleasePoolID(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	_, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	alloc1, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8"})
	if err != nil {
		t.Fatalf("Unexpected failure in adding pool: %v", err)
	}
	k0, err := PoolIDFromString(alloc1.PoolID)
	if err != nil {
		t.Fatal(err)
	}

	aSpace, err := a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := aSpace.subnets[k0.Subnet].autoRelease; got != false {
		t.Errorf("Unexpected autoRelease value for %s: %v", k0, got)
	}

	alloc2, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8", SubPool: "10.0.0.0/16"})
	if err != nil {
		t.Fatalf("Unexpected failure in adding sub pool: %v", err)
	}
	k1, err := PoolIDFromString(alloc2.PoolID)
	if err != nil {
		t.Fatal(err)
	}

	if alloc1.PoolID == alloc2.PoolID {
		t.Fatalf("Incorrect poolIDs returned %s, %s", alloc1.PoolID, alloc2.PoolID)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := aSpace.subnets[k1.Subnet].autoRelease; got != false {
		t.Errorf("Unexpected autoRelease value for %s: %v", k1, got)
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8", SubPool: "10.0.0.0/16"})
	if err == nil {
		t.Fatalf("Expected failure in adding sub pool: %v", err)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := aSpace.subnets[k0.Subnet].autoRelease; got != false {
		t.Errorf("Unexpected autoRelease value for %s: %v", k0, got)
	}

	if err := a.ReleasePool(alloc2.PoolID); err != nil {
		t.Fatal(err)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := aSpace.subnets[k0.Subnet].autoRelease; got != false {
		t.Errorf("Unexpected autoRelease value for %s: %v", k0, got)
	}
	if err := a.ReleasePool(alloc1.PoolID); err != nil {
		t.Error(err)
	}

	if _, ok := aSpace.subnets[k0.Subnet]; ok {
		t.Error("Pool should have been deleted when released")
	}

	alloc10, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8"})
	if err != nil {
		t.Errorf("Unexpected failure in adding pool: %v", err)
	}
	if alloc10.PoolID != alloc1.PoolID {
		t.Errorf("main pool should still exist. Got poolID %q, want %q", alloc10.PoolID, alloc1.PoolID)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := aSpace.subnets[k0.Subnet].autoRelease; got != false {
		t.Errorf("Unexpected autoRelease value for %s: %v", k0, got)
	}

	if err := a.ReleasePool(alloc10.PoolID); err != nil {
		t.Error(err)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if bp, ok := aSpace.subnets[k0.Subnet]; ok {
		t.Errorf("Base pool %s is still present: %v", k0, bp)
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8"})
	if err != nil {
		t.Errorf("Unexpected failure in adding pool: %v", err)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := aSpace.subnets[k0.Subnet].autoRelease; got != false {
		t.Errorf("Unexpected autoRelease value for %s: %v", k0, got)
	}
}

func TestPredefinedPool(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc1, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
	if err != nil {
		t.Fatal(err)
	}

	alloc2, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
	if err != nil {
		t.Fatal(err)
	}

	if alloc1.Pool == alloc2.Pool {
		t.Fatalf("Unexpected default network returned: %s = %s", alloc2.Pool, alloc1.Pool)
	}

	if err := a.ReleasePool(alloc1.PoolID); err != nil {
		t.Fatal(err)
	}

	if err := a.ReleasePool(alloc2.PoolID); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveSubnet(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	reqs := []ipamapi.PoolRequest{
		{AddressSpace: localAddressSpace, Pool: "192.168.0.0/16"},
		{AddressSpace: localAddressSpace, Pool: "172.17.0.0/16"},
		{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8"},
		{AddressSpace: localAddressSpace, Pool: "2001:db8:1:2:3:4:ffff::/112", V6: true},
		{AddressSpace: globalAddressSpace, Pool: "172.17.0.0/16"},
		{AddressSpace: globalAddressSpace, Pool: "10.0.0.0/8"},
		{AddressSpace: globalAddressSpace, Pool: "2001:db8:1:2:3:4:5::/112", V6: true},
		{AddressSpace: globalAddressSpace, Pool: "2001:db8:1:2:3:4:ffff::/112", V6: true},
	}
	allocs := make([]ipamapi.AllocatedPool, 0, len(reqs))

	for _, req := range reqs {
		alloc, err := a.RequestPool(req)
		if err != nil {
			t.Fatalf("Failed to apply input. Can't proceed: %s", err.Error())
		}
		allocs = append(allocs, alloc)
	}

	for idx, alloc := range allocs {
		if err := a.ReleasePool(alloc.PoolID); err != nil {
			t.Fatalf("Failed to release poolID %s (%d)", alloc.PoolID, idx)
		}
	}
}

func TestGetSameAddress(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "192.168.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}

	ip := net.ParseIP("192.168.100.250")
	_, _, err = a.RequestAddress(alloc.PoolID, ip, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = a.RequestAddress(alloc.PoolID, ip, nil)
	if err == nil {
		t.Fatal(err)
	}
}

func TestPoolAllocationReuse(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	// First get all pools until they are exhausted to
	allocs := []ipamapi.AllocatedPool{}
	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
	for err == nil {
		allocs = append(allocs, alloc)
		alloc, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
	}
	for _, alloc := range allocs {
		if err := a.ReleasePool(alloc.PoolID); err != nil {
			t.Fatal(err)
		}
	}

	// Now try to allocate then free nPool pools sequentially.
	// Verify that we don't see any repeat networks even though
	// we have freed them.
	seen := map[string]bool{}
	for range allocs {
		alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := seen[alloc.Pool.String()]; ok {
			t.Fatalf("Network %s was reused before exhausing the pool list", alloc.Pool.String())
		}
		seen[alloc.Pool.String()] = true
		if err := a.ReleasePool(alloc.PoolID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetAddressSubPoolEqualPool(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	// Requesting a subpool of same size of the master pool should not cause any problem on ip allocation
	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "172.18.0.0/16", SubPool: "172.18.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = a.RequestAddress(alloc.PoolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestReleaseAddressFromSubPool(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "172.28.0.0/16", SubPool: "172.28.30.0/24"})
	if err != nil {
		t.Fatal(err)
	}

	var ip *net.IPNet
	expected := &net.IPNet{IP: net.IP{172, 28, 30, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(alloc.PoolID, nil, nil); err == nil {
			ip = c
		}
	}
	if err != ipamapi.ErrNoAvailableIPs {
		t.Fatal(err)
	}
	if !types.CompareIPNet(expected, ip) {
		t.Fatalf("Unexpected last IP from subpool. Expected: %s. Got: %v.", expected, ip)
	}
	rp := &net.IPNet{IP: net.IP{172, 28, 30, 97}, Mask: net.IPMask{255, 255, 0, 0}}
	if err = a.ReleaseAddress(alloc.PoolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(alloc.PoolID, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8", SubPool: "10.0.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	alloc, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/16", SubPool: "10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	expected = &net.IPNet{IP: net.IP{10, 0, 0, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(alloc.PoolID, nil, nil); err == nil {
			ip = c
		}
	}
	if err != ipamapi.ErrNoAvailableIPs {
		t.Fatal(err)
	}
	if !types.CompareIPNet(expected, ip) {
		t.Fatalf("Unexpected last IP from subpool. Expected: %s. Got: %v.", expected, ip)
	}
	rp = &net.IPNet{IP: net.IP{10, 0, 0, 79}, Mask: net.IPMask{255, 255, 0, 0}}
	if err = a.ReleaseAddress(alloc.PoolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(alloc.PoolID, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	// Request any addresses from subpool after explicit address request
	unoExp, _ := types.ParseCIDR("10.2.2.0/16")
	dueExp, _ := types.ParseCIDR("10.2.2.2/16")
	treExp, _ := types.ParseCIDR("10.2.2.1/16")

	if alloc, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.2.0.0/16", SubPool: "10.2.2.0/24"}); err != nil {
		t.Fatal(err)
	}
	tre, _, err := a.RequestAddress(alloc.PoolID, treExp.IP, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, treExp) {
		t.Fatalf("Unexpected address: want %v, got %v", treExp, tre)
	}

	uno, _, err := a.RequestAddress(alloc.PoolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, unoExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	due, _, err := a.RequestAddress(alloc.PoolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(due, dueExp) {
		t.Fatalf("Unexpected address: %v", due)
	}

	if err = a.ReleaseAddress(alloc.PoolID, uno.IP); err != nil {
		t.Fatal(err)
	}
	uno, _, err = a.RequestAddress(alloc.PoolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, unoExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	if err = a.ReleaseAddress(alloc.PoolID, tre.IP); err != nil {
		t.Fatal(err)
	}
	tre, _, err = a.RequestAddress(alloc.PoolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, treExp) {
		t.Fatalf("Unexpected address: %v", tre)
	}
}

func TestSerializeRequestReleaseAddressFromSubPool(t *testing.T) {
	opts := map[string]string{
		ipamapi.AllocSerialPrefix: "true",
	}
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "172.28.0.0/16", SubPool: "172.28.30.0/24"})
	if err != nil {
		t.Fatal(err)
	}

	var ip *net.IPNet
	expected := &net.IPNet{IP: net.IP{172, 28, 30, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(alloc.PoolID, nil, opts); err == nil {
			ip = c
		}
	}
	if err != ipamapi.ErrNoAvailableIPs {
		t.Fatal(err)
	}
	if !types.CompareIPNet(expected, ip) {
		t.Fatalf("Unexpected last IP from subpool. Expected: %s. Got: %v.", expected, ip)
	}
	rp := &net.IPNet{IP: net.IP{172, 28, 30, 97}, Mask: net.IPMask{255, 255, 0, 0}}
	if err = a.ReleaseAddress(alloc.PoolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(alloc.PoolID, nil, opts); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/8", SubPool: "10.0.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	alloc, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.0.0.0/16", SubPool: "10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	expected = &net.IPNet{IP: net.IP{10, 0, 0, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(alloc.PoolID, nil, opts); err == nil {
			ip = c
		}
	}
	if err != ipamapi.ErrNoAvailableIPs {
		t.Fatal(err)
	}
	if !types.CompareIPNet(expected, ip) {
		t.Fatalf("Unexpected last IP from subpool. Expected: %s. Got: %v.", expected, ip)
	}
	rp = &net.IPNet{IP: net.IP{10, 0, 0, 79}, Mask: net.IPMask{255, 255, 0, 0}}
	if err = a.ReleaseAddress(alloc.PoolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(alloc.PoolID, nil, opts); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	// Request any addresses from subpool after explicit address request
	unoExp, _ := types.ParseCIDR("10.2.2.0/16")
	dueExp, _ := types.ParseCIDR("10.2.2.2/16")
	treExp, _ := types.ParseCIDR("10.2.2.1/16")
	quaExp, _ := types.ParseCIDR("10.2.2.3/16")
	fivExp, _ := types.ParseCIDR("10.2.2.4/16")
	if alloc, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "10.2.0.0/16", SubPool: "10.2.2.0/24"}); err != nil {
		t.Fatal(err)
	}
	tre, _, err := a.RequestAddress(alloc.PoolID, treExp.IP, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, treExp) {
		t.Fatalf("Unexpected address: want %v, got %v", treExp, tre)
	}

	uno, _, err := a.RequestAddress(alloc.PoolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, unoExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	due, _, err := a.RequestAddress(alloc.PoolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(due, dueExp) {
		t.Fatalf("Unexpected address: %v", due)
	}

	if err = a.ReleaseAddress(alloc.PoolID, uno.IP); err != nil {
		t.Fatal(err)
	}
	uno, _, err = a.RequestAddress(alloc.PoolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, quaExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	if err = a.ReleaseAddress(alloc.PoolID, tre.IP); err != nil {
		t.Fatal(err)
	}
	tre, _, err = a.RequestAddress(alloc.PoolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, fivExp) {
		t.Fatalf("Unexpected address: %v", tre)
	}
}

func TestGetAddress(t *testing.T) {
	input := []string{
		/*"10.0.0.0/8", "10.0.0.0/9", "10.0.0.0/10",*/ "10.0.0.0/11", "10.0.0.0/12", "10.0.0.0/13", "10.0.0.0/14",
		"10.0.0.0/15", "10.0.0.0/16", "10.0.0.0/17", "10.0.0.0/18", "10.0.0.0/19", "10.0.0.0/20", "10.0.0.0/21",
		"10.0.0.0/22", "10.0.0.0/23", "10.0.0.0/24", "10.0.0.0/25", "10.0.0.0/26", "10.0.0.0/27", "10.0.0.0/28",
		"10.0.0.0/29", "10.0.0.0/30", "10.0.0.0/31",
	}

	for _, subnet := range input {
		assertGetAddress(t, subnet)
	}
}

func TestRequestSyntaxCheck(t *testing.T) {
	var (
		pool    = "192.168.0.0/16"
		subPool = "192.168.0.0/24"
	)

	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	_, err = a.RequestPool(ipamapi.PoolRequest{Pool: pool})
	if err == nil {
		t.Fatal("Failed to detect wrong request: empty address space")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{Pool: pool, SubPool: subPool})
	if err == nil {
		t.Fatal("Failed to detect wrong request: empty address space")
	}

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, SubPool: subPool})
	if err == nil {
		t.Fatal("Failed to detect wrong request: subPool specified and no pool")
	}

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: pool, SubPool: subPool})
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	_, _, err = a.RequestAddress("", nil, nil)
	if err == nil {
		t.Fatal("Failed to detect wrong request: no pool id specified")
	}

	ip := net.ParseIP("172.17.0.23")
	_, _, err = a.RequestAddress(alloc.PoolID, ip, nil)
	if err == nil {
		t.Fatal("Failed to detect wrong request: requested IP from different subnet")
	}

	ip = net.ParseIP("192.168.0.50")
	_, _, err = a.RequestAddress(alloc.PoolID, ip, nil)
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	err = a.ReleaseAddress("", ip)
	if err == nil {
		t.Fatal("Failed to detect wrong request: no pool id specified")
	}

	err = a.ReleaseAddress(alloc.PoolID, nil)
	if err == nil {
		t.Fatal("Failed to detect wrong request: no pool id specified")
	}

	err = a.ReleaseAddress(alloc.PoolID, ip)
	if err != nil {
		t.Fatalf("Unexpected failure: %v: %s, %s", err, alloc.PoolID, ip)
	}
}

func TestRequest(t *testing.T) {
	// Request N addresses from different size subnets, verifying last request
	// returns expected address. Internal subnet host size is Allocator's default, 16
	input := []struct {
		subnet string
		numReq int
		lastIP string
	}{
		{"192.168.59.0/24", 254, "192.168.59.254"},
		{"192.168.240.0/20", 255, "192.168.240.255"},
		{"192.168.0.0/16", 255, "192.168.0.255"},
		{"192.168.0.0/16", 256, "192.168.1.0"},
		{"10.16.0.0/16", 255, "10.16.0.255"},
		{"10.128.0.0/12", 255, "10.128.0.255"},
		{"10.0.0.0/8", 256, "10.0.1.0"},

		{"192.168.128.0/18", 4*256 - 1, "192.168.131.255"},
		/*
			{"192.168.240.0/20", 16*256 - 2, "192.168.255.254"},

			{"192.168.0.0/16", 256*256 - 2, "192.168.255.254"},
			{"10.0.0.0/8", 2 * 256, "10.0.2.0"},
			{"10.0.0.0/8", 5 * 256, "10.0.5.0"},
			{"10.0.0.0/8", 100 * 256 * 254, "10.99.255.254"},
		*/
	}

	for _, d := range input {
		assertNRequests(t, d.subnet, d.numReq, d.lastIP)
	}
}

// TestOverlappingRequests tests that overlapping subnets cannot be allocated.
// Requests for subnets which are supersets or subsets of existing allocations,
// or which overlap at the beginning or end, should not be permitted.
func TestOverlappingRequests(t *testing.T) {
	input := []struct {
		environment []string
		subnet      string
		ok          bool
	}{
		// IPv4
		// Previously allocated network does not overlap with request
		{[]string{"10.0.0.0/8"}, "11.0.0.0/8", true},
		{[]string{"74.0.0.0/7"}, "9.111.99.72/30", true},
		{[]string{"110.192.0.0/10"}, "16.0.0.0/10", true},

		// Previously allocated network entirely contains request
		{[]string{"10.0.0.0/8"}, "10.0.0.0/8", false}, // exact overlap
		{[]string{"0.0.0.0/1"}, "16.182.0.0/15", false},
		{[]string{"16.0.0.0/4"}, "17.11.66.0/23", false},

		// Previously allocated network overlaps beginning of request
		{[]string{"0.0.0.0/1"}, "0.0.0.0/0", false},
		{[]string{"64.0.0.0/6"}, "64.0.0.0/3", false},
		{[]string{"112.0.0.0/6"}, "112.0.0.0/4", false},

		// Previously allocated network overlaps end of request
		{[]string{"96.0.0.0/3"}, "0.0.0.0/1", false},
		{[]string{"192.0.0.0/2"}, "128.0.0.0/1", false},
		{[]string{"95.0.0.0/8"}, "92.0.0.0/6", false},

		// Previously allocated network entirely contained within request
		{[]string{"10.0.0.0/8"}, "10.0.0.0/6", false}, // non-canonical
		{[]string{"10.0.0.0/8"}, "8.0.0.0/6", false},  // canonical
		{[]string{"25.173.144.0/20"}, "0.0.0.0/0", false},

		// IPv6
		// Previously allocated network entirely contains request
		{[]string{"::/0"}, "f656:3484:c878:a05:e540:a6ed:4d70:3740/123", false},
		{[]string{"8000::/1"}, "8fe8:e7c4:5779::/49", false},
		{[]string{"f000::/4"}, "ffc7:6000::/19", false},

		// Previously allocated network overlaps beginning of request
		{[]string{"::/2"}, "::/0", false},
		{[]string{"::/3"}, "::/1", false},
		{[]string{"::/6"}, "::/5", false},

		// Previously allocated network overlaps end of request
		{[]string{"c000::/2"}, "8000::/1", false},
		{[]string{"7c00::/6"}, "::/1", false},
		{[]string{"cf80::/9"}, "c000::/4", false},

		// Previously allocated network entirely contained within request
		{[]string{"ff77:93f8::/29"}, "::/0", false},
		{[]string{"9287:2e20:5134:fab6:9061:a0c6:bfe3:9400/119"}, "8000::/1", false},
		{[]string{"3ea1:bfa9:8691:d1c6:8c46:519b:db6d:e700/120"}, "3000::/4", false},
	}

	for _, tc := range input {
		a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
		assert.NilError(t, err)

		// Set up some existing allocations.  This should always succeed.
		for _, env := range tc.environment {
			_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: env})
			assert.NilError(t, err)
		}

		// Make the test allocation.
		_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: tc.subnet})
		if tc.ok {
			assert.NilError(t, err)
		} else {
			assert.Check(t, is.ErrorContains(err, ""))
		}
	}
}

func TestUnusualSubnets(t *testing.T) {
	subnet := "192.168.0.2/31"

	outsideTheRangeAddresses := []struct {
		address string
	}{
		{"192.168.0.1"},
		{"192.168.0.4"},
		{"192.168.0.100"},
	}

	expectedAddresses := []struct {
		address string
	}{
		{"192.168.0.2"},
		{"192.168.0.3"},
	}

	allocator, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}

	//
	// IPv4 /31 blocks.  See RFC 3021.
	//

	alloc, err := allocator.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: subnet})
	if err != nil {
		t.Fatal(err)
	}

	// Outside-the-range

	for _, outside := range outsideTheRangeAddresses {
		_, _, errx := allocator.RequestAddress(alloc.PoolID, net.ParseIP(outside.address), nil)
		if errx != ipamapi.ErrIPOutOfRange {
			t.Fatalf("Address %s failed to throw expected error: %s", outside.address, errx.Error())
		}
	}

	// Should get just these two IPs followed by exhaustion on the next request

	for _, expected := range expectedAddresses {
		got, _, errx := allocator.RequestAddress(alloc.PoolID, nil, nil)
		if errx != nil {
			t.Fatalf("Failed to obtain the address: %s", errx.Error())
		}
		expectedIP := net.ParseIP(expected.address)
		gotIP := got.IP
		if !gotIP.Equal(expectedIP) {
			t.Fatalf("Failed to obtain sequentialaddress. Expected: %s, Got: %s", expectedIP, gotIP)
		}
	}

	_, _, err = allocator.RequestAddress(alloc.PoolID, nil, nil)
	if err != ipamapi.ErrNoAvailableIPs {
		t.Fatal("Did not get expected error when pool is exhausted.")
	}
}

func TestRelease(t *testing.T) {
	subnet := "192.168.0.0/23"

	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: subnet})
	if err != nil {
		t.Fatal(err)
	}

	// Allocate all addresses
	for err != ipamapi.ErrNoAvailableIPs {
		_, _, err = a.RequestAddress(alloc.PoolID, nil, nil)
	}

	toRelease := []struct {
		address string
	}{
		{"192.168.0.1"},
		{"192.168.0.2"},
		{"192.168.0.3"},
		{"192.168.0.4"},
		{"192.168.0.5"},
		{"192.168.0.6"},
		{"192.168.0.7"},
		{"192.168.0.8"},
		{"192.168.0.9"},
		{"192.168.0.10"},
		{"192.168.0.30"},
		{"192.168.0.31"},
		{"192.168.1.32"},

		{"192.168.0.254"},
		{"192.168.1.1"},
		{"192.168.1.2"},

		{"192.168.1.3"},

		{"192.168.1.253"},
		{"192.168.1.254"},
	}

	// One by one, release the address and request again. We should get the same IP
	for i, inp := range toRelease {
		ip0 := net.ParseIP(inp.address)
		a.ReleaseAddress(alloc.PoolID, ip0)
		bm := a.local4.subnets[netip.MustParsePrefix(subnet)].addrs
		if bm.Unselected() != 1 {
			t.Fatalf("Failed to update free address count after release. Expected %d, Found: %d", i+1, bm.Unselected())
		}

		nw, _, err := a.RequestAddress(alloc.PoolID, nil, nil)
		if err != nil {
			t.Fatalf("Failed to obtain the address: %s", err.Error())
		}
		ip := nw.IP
		if !ip0.Equal(ip) {
			t.Fatalf("Failed to obtain the same address. Expected: %s, Got: %s", ip0, ip)
		}
	}
}

func assertGetAddress(t *testing.T, subnet string) {
	var (
		err       error
		printTime = false
	)

	sub := netip.MustParsePrefix(subnet)
	ones, bits := sub.Bits(), sub.Addr().BitLen()
	zeroes := bits - ones
	numAddresses := 1 << uint(zeroes)

	bm := bitmap.New(uint64(numAddresses))

	start := time.Now()
	run := 0
	for err != ipamapi.ErrNoAvailableIPs {
		_, err = getAddress(sub, bm, netip.Addr{}, netip.Prefix{}, false)
		run++
	}
	if printTime {
		fmt.Printf("\nTaken %v, to allocate all addresses on %s. (nemAddresses: %d. Runs: %d)", time.Since(start), subnet, numAddresses, run)
	}
	if bm.Unselected() != 0 {
		t.Fatalf("Unexpected free count after reserving all addresses: %d", bm.Unselected())
	}
	/*
		if bm.Head.Block != expectedMax || bm.Head.Count != numBlocks {
			t.Fatalf("Failed to effectively reserve all addresses on %s. Expected (0x%x, %d) as first sequence. Found (0x%x,%d)",
				subnet, expectedMax, numBlocks, bm.Head.Block, bm.Head.Count)
		}
	*/
}

func assertNRequests(t *testing.T, subnet string, numReq int, lastExpectedIP string) {
	var (
		nw        *net.IPNet
		printTime = false
	)

	lastIP := net.ParseIP(lastExpectedIP)
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: subnet})
	if err != nil {
		t.Fatal(err)
	}

	i := 0
	start := time.Now()
	for ; i < numReq; i++ {
		nw, _, err = a.RequestAddress(alloc.PoolID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	if printTime {
		fmt.Printf("\nTaken %v, to allocate %d addresses on %s\n", time.Since(start), numReq, subnet)
	}

	if !lastIP.Equal(nw.IP) {
		t.Fatalf("Wrong last IP. Expected %s. Got: %s (err: %v, ind: %d)", lastExpectedIP, nw.IP.String(), err, i)
	}
}

func benchmarkRequest(b *testing.B, a *Allocator, subnet string) {
	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: subnet})
	for err != ipamapi.ErrNoAvailableIPs {
		_, _, err = a.RequestAddress(alloc.PoolID, nil, nil)
	}
}

func BenchmarkRequest(b *testing.B) {
	subnets := []string{
		"10.0.0.0/24",
		"10.0.0.0/16",
		"10.0.0.0/8",
	}

	for _, subnet := range subnets {
		name := fmt.Sprintf("%vSubnet", subnet)
		b.Run(name, func(b *testing.B) {
			a, _ := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
			benchmarkRequest(b, a, subnet)
		})
	}
}

func TestAllocateRandomDeallocate(t *testing.T) {
	for _, store := range []bool{false, true} {
		testAllocateRandomDeallocate(t, "172.25.0.0/16", "", 384, store)
		testAllocateRandomDeallocate(t, "172.25.0.0/16", "172.25.252.0/22", 384, store)
	}
}

func testAllocateRandomDeallocate(t *testing.T, pool, subPool string, num int, store bool) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: pool, SubPool: subPool})
	if err != nil {
		t.Fatal(err)
	}

	// Allocate num ip addresses
	indices := make(map[int]*net.IPNet, num)
	allocated := make(map[string]bool, num)
	for i := 0; i < num; i++ {
		ip, _, err := a.RequestAddress(alloc.PoolID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ips := ip.String()
		if _, ok := allocated[ips]; ok {
			t.Fatalf("Address %s is already allocated", ips)
		}
		allocated[ips] = true
		indices[i] = ip
	}
	if len(indices) != len(allocated) || len(indices) != num {
		t.Fatalf("Unexpected number of allocated addresses: (%d,%d).", len(indices), len(allocated))
	}

	seed := time.Now().Unix()
	rng := rand.New(rand.NewSource(seed))

	// Deallocate half of the allocated addresses following a random pattern
	pattern := rng.Perm(num)
	for i := 0; i < num/2; i++ {
		idx := pattern[i]
		ip := indices[idx]
		err := a.ReleaseAddress(alloc.PoolID, ip.IP)
		if err != nil {
			t.Fatalf("Unexpected failure on deallocation of %s: %v.\nSeed: %d.", ip, err, seed)
		}
		delete(indices, idx)
		delete(allocated, ip.String())
	}

	// Request a quarter of addresses
	for i := 0; i < num/2; i++ {
		ip, _, err := a.RequestAddress(alloc.PoolID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ips := ip.String()
		if _, ok := allocated[ips]; ok {
			t.Fatalf("\nAddress %s is already allocated.\nSeed: %d.", ips, seed)
		}
		allocated[ips] = true
	}
	if len(allocated) != num {
		t.Fatalf("Unexpected number of allocated addresses: %d.\nSeed: %d.", len(allocated), seed)
	}
}

const (
	numInstances = 5
	first        = 0
)

var (
	allocator *Allocator
	start     = make(chan struct{})
	done      sync.WaitGroup
	pools     = make([]*net.IPNet, numInstances)
)

func runParallelTests(t *testing.T, instance int) {
	var err error

	t.Parallel()

	pTest := flag.Lookup("test.parallel")
	if pTest == nil {
		t.Skip("Skipped because test.parallel flag not set;")
	}
	numParallel, err := strconv.Atoi(pTest.Value.String())
	if err != nil {
		t.Fatal(err)
	}
	if numParallel < numInstances {
		t.Skip("Skipped because t.parallel was less than ", numInstances)
	}

	// The first instance creates the allocator, gives the start
	// and finally checks the pools each instance was assigned
	if instance == first {
		allocator, err = NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
		if err != nil {
			t.Fatal(err)
		}
		done.Add(numInstances - 1)
		close(start)
	}

	if instance != first {
		<-start
		defer done.Done()
	}

	alloc, err := allocator.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
	pools[instance] = netiputil.ToIPNet(alloc.Pool)
	if err != nil {
		t.Fatal(err)
	}

	if instance == first {
		done.Wait()
		// Now check each instance got a different pool
		for i := 0; i < numInstances; i++ {
			for j := i + 1; j < numInstances; j++ {
				if types.CompareIPNet(pools[i], pools[j]) {
					t.Errorf("Instance %d and %d were given the same predefined pool: %v", i, j, pools)
				}
			}
		}
	}
}

func TestRequestReleaseAddressDuplicate(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}
	type IP struct {
		ip  *net.IPNet
		ref int
	}
	ips := []IP{}
	allocatedIPs := []*net.IPNet{}

	opts := map[string]string{
		ipamapi.AllocSerialPrefix: "true",
	}
	var l sync.Mutex

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "198.168.0.0/23"})
	if err != nil {
		t.Fatal(err)
	}

	seed := time.Now().Unix()
	t.Logf("Random seed: %v", seed)
	rng := rand.New(rand.NewSource(seed))

	group, ctx := errgroup.WithContext(context.Background())
outer:
	for n := 0; n < 10000; n++ {
		var c *net.IPNet
		for {
			select {
			case <-ctx.Done():
				// One of group's goroutines returned an error.
				break outer
			default:
			}
			if c, _, err = a.RequestAddress(alloc.PoolID, nil, opts); err == nil {
				break
			}
			// No addresses available. Spin until one is.
			runtime.Gosched()
		}
		l.Lock()
		ips = append(ips, IP{c, 1})
		l.Unlock()
		allocatedIPs = append(allocatedIPs, c)
		if len(allocatedIPs) > 500 {
			i := rng.Intn(len(allocatedIPs) - 1)
			ip := allocatedIPs[i]
			allocatedIPs = append(allocatedIPs[:i], allocatedIPs[i+1:]...)

			group.Go(func() error {
				// The lifetime of an allocated address begins when RequestAddress returns, and
				// ends when ReleaseAddress is called. But we can't atomically call one of those
				// methods and append to the log (ips slice) without also synchronizing the
				// calls with each other. Synchronizing the calls would defeat the whole point
				// of this test, which is to race ReleaseAddress against RequestAddress. We have
				// no choice but to leave a small window of uncertainty open. Appending to the
				// log after ReleaseAddress returns would allow the next RequestAddress call to
				// race the log-release operation, which could result in the reallocate being
				// logged before the release, despite the release happening before the
				// reallocate: a false positive. Our only other option is to append the release
				// to the log before calling ReleaseAddress, leaving a small race window for
				// false negatives. False positives mean a flaky test, so let's err on the side
				// of false negatives. Eventually we'll get lucky with a true-positive test
				// failure or with Go's race detector if a concurrency bug exists.
				l.Lock()
				ips = append(ips, IP{ip, -1})
				l.Unlock()
				return a.ReleaseAddress(alloc.PoolID, ip.IP)
			})
		}
	}

	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}

	refMap := make(map[string]int)
	for _, ip := range ips {
		refMap[ip.ip.String()] = refMap[ip.ip.String()] + ip.ref
		if refMap[ip.ip.String()] < 0 {
			t.Fatalf("IP %s was previously released", ip.ip.String())
		}
		if refMap[ip.ip.String()] > 1 {
			t.Fatalf("IP %s was previously allocated", ip.ip.String())
		}
	}
}

func TestParallelPredefinedRequest1(t *testing.T) {
	runParallelTests(t, 0)
}

func TestParallelPredefinedRequest2(t *testing.T) {
	runParallelTests(t, 1)
}

func TestParallelPredefinedRequest3(t *testing.T) {
	runParallelTests(t, 2)
}

func TestParallelPredefinedRequest4(t *testing.T) {
	runParallelTests(t, 3)
}

func TestParallelPredefinedRequest5(t *testing.T) {
	runParallelTests(t, 4)
}

func BenchmarkPoolIDToString(b *testing.B) {
	const poolIDString = "default/172.27.0.0/16/172.27.3.0/24"
	k, err := PoolIDFromString(poolIDString)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = k.String()
	}
}

func BenchmarkPoolIDFromString(b *testing.B) {
	const poolIDString = "default/172.27.0.0/16/172.27.3.0/24"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := PoolIDFromString(poolIDString)
		if err != nil {
			b.Fatal(err)
		}
	}
}
