package ipam

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/libnetwork/bitseq"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/types"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInt2IP2IntConversion(t *testing.T) {
	for i := uint64(0); i < 256*256*256; i++ {
		var array [4]byte // new array at each cycle
		addIntToIP(array[:], i)
		j := ipToUint64(array[:])
		if j != i {
			t.Fatalf("Failed to convert ordinal %d to IP % x and back to ordinal. Got %d", i, array, j)
		}
	}
}

func TestGetAddressVersion(t *testing.T) {
	if v4 != getAddressVersion(net.ParseIP("172.28.30.112")) {
		t.Fatal("Failed to detect IPv4 version")
	}
	if v4 != getAddressVersion(net.ParseIP("0.0.0.1")) {
		t.Fatal("Failed to detect IPv4 version")
	}
	if v6 != getAddressVersion(net.ParseIP("ff01::1")) {
		t.Fatal("Failed to detect IPv6 version")
	}
	if v6 != getAddressVersion(net.ParseIP("2001:db8::76:51")) {
		t.Fatal("Failed to detect IPv6 version")
	}
}

func TestKeyString(t *testing.T) {
	k := &SubnetKey{AddressSpace: "default", Subnet: "172.27.0.0/16"}
	expected := "default/172.27.0.0/16"
	if expected != k.String() {
		t.Fatalf("Unexpected key string: %s", k.String())
	}

	k2 := &SubnetKey{}
	err := k2.FromString(expected)
	if err != nil {
		t.Fatal(err)
	}
	if k2.AddressSpace != k.AddressSpace || k2.Subnet != k.Subnet {
		t.Fatalf("SubnetKey.FromString() failed. Expected %v. Got %v", k, k2)
	}

	expected = fmt.Sprintf("%s/%s", expected, "172.27.3.0/24")
	k.ChildSubnet = "172.27.3.0/24"
	if expected != k.String() {
		t.Fatalf("Unexpected key string: %s", k.String())
	}

	err = k2.FromString(expected)
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
	a.addrSpaces["abc"] = a.addrSpaces[localAddressSpace]

	pid0, _, _, err := a.RequestPool(localAddressSpace, "10.0.0.0/8", "", nil, false)
	if err != nil {
		t.Fatal("Unexpected failure in adding subnet")
	}

	pid1, _, _, err := a.RequestPool("abc", "10.0.0.0/8", "", nil, false)
	if err != nil {
		t.Fatalf("Unexpected failure in adding overlapping subnets to different address spaces: %v", err)
	}

	if pid0 == pid1 {
		t.Fatal("returned same pool id for same subnets in different namespaces")
	}

	_, _, _, err = a.RequestPool("abc", "10.0.0.0/8", "", nil, false)
	if err == nil {
		t.Fatalf("Expected failure requesting existing subnet")
	}

	_, _, _, err = a.RequestPool("abc", "10.128.0.0/9", "", nil, false)
	if err == nil {
		t.Fatal("Expected failure on adding overlapping base subnet")
	}

	_, _, _, err = a.RequestPool("abc", "10.0.0.0/8", "10.128.0.0/9", nil, false)
	if err != nil {
		t.Fatalf("Unexpected failure on adding sub pool: %v", err)
	}
	_, _, _, err = a.RequestPool("abc", "10.0.0.0/8", "10.128.0.0/9", nil, false)
	if err == nil {
		t.Fatalf("Expected failure on adding overlapping sub pool")
	}

	_, _, _, err = a.RequestPool(localAddressSpace, "10.20.2.0/24", "", nil, false)
	if err == nil {
		t.Fatal("Failed to detect overlapping subnets")
	}

	_, _, _, err = a.RequestPool(localAddressSpace, "10.128.0.0/9", "", nil, false)
	if err == nil {
		t.Fatal("Failed to detect overlapping subnets")
	}

	_, _, _, err = a.RequestPool(localAddressSpace, "1003:1:2:3:4:5:6::/112", "", nil, false)
	if err != nil {
		t.Fatalf("Failed to add v6 subnet: %s", err.Error())
	}

	_, _, _, err = a.RequestPool(localAddressSpace, "1003:1:2:3::/64", "", nil, false)
	if err == nil {
		t.Fatal("Failed to detect overlapping v6 subnet")
	}
}

// TestDoublePoolRelease tests that releasing a pool which has already
// been released raises an error.
func TestDoublePoolRelease(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	pid0, _, _, err := a.RequestPool(localAddressSpace, "10.0.0.0/8", "", nil, false)
	assert.NilError(t, err)

	err = a.ReleasePool(pid0)
	assert.NilError(t, err)

	err = a.ReleasePool(pid0)
	assert.Check(t, is.ErrorContains(err, ""))
}

func TestAddReleasePoolID(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	var k0, k1 SubnetKey
	_, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	pid0, _, _, err := a.RequestPool(localAddressSpace, "10.0.0.0/8", "", nil, false)
	if err != nil {
		t.Fatal("Unexpected failure in adding pool")
	}
	if err := k0.FromString(pid0); err != nil {
		t.Fatal(err)
	}

	aSpace, err := a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets := aSpace.subnets

	if subnets[k0].RefCount != 1 {
		t.Fatalf("Unexpected ref count for %s: %d", k0, subnets[k0].RefCount)
	}

	pid1, _, _, err := a.RequestPool(localAddressSpace, "10.0.0.0/8", "10.0.0.0/16", nil, false)
	if err != nil {
		t.Fatal("Unexpected failure in adding sub pool")
	}
	if err := k1.FromString(pid1); err != nil {
		t.Fatal(err)
	}

	if pid0 == pid1 {
		t.Fatalf("Incorrect poolIDs returned %s, %s", pid0, pid1)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets = aSpace.subnets
	if subnets[k1].RefCount != 1 {
		t.Fatalf("Unexpected ref count for %s: %d", k1, subnets[k1].RefCount)
	}

	_, _, _, err = a.RequestPool(localAddressSpace, "10.0.0.0/8", "10.0.0.0/16", nil, false)
	if err == nil {
		t.Fatal("Expected failure in adding sub pool")
	}

	aSpace, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets = aSpace.subnets

	if subnets[k0].RefCount != 2 {
		t.Fatalf("Unexpected ref count for %s: %d", k0, subnets[k0].RefCount)
	}

	if err := a.ReleasePool(pid1); err != nil {
		t.Fatal(err)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets = aSpace.subnets
	if subnets[k0].RefCount != 1 {
		t.Fatalf("Unexpected ref count for %s: %d", k0, subnets[k0].RefCount)
	}
	if err := a.ReleasePool(pid0); err != nil {
		t.Fatal(err)
	}

	pid00, _, _, err := a.RequestPool(localAddressSpace, "10.0.0.0/8", "", nil, false)
	if err != nil {
		t.Fatal("Unexpected failure in adding pool")
	}
	if pid00 != pid0 {
		t.Fatal("main pool should still exist")
	}

	aSpace, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets = aSpace.subnets
	if subnets[k0].RefCount != 1 {
		t.Fatalf("Unexpected ref count for %s: %d", k0, subnets[k0].RefCount)
	}

	if err := a.ReleasePool(pid00); err != nil {
		t.Fatal(err)
	}

	aSpace, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets = aSpace.subnets
	if bp, ok := subnets[k0]; ok {
		t.Fatalf("Base pool %s is still present: %v", k0, bp)
	}

	_, _, _, err = a.RequestPool(localAddressSpace, "10.0.0.0/8", "", nil, false)
	if err != nil {
		t.Fatal("Unexpected failure in adding pool")
	}

	aSpace, err = a.getAddrSpace(localAddressSpace)
	if err != nil {
		t.Fatal(err)
	}

	subnets = aSpace.subnets
	if subnets[k0].RefCount != 1 {
		t.Fatalf("Unexpected ref count for %s: %d", k0, subnets[k0].RefCount)
	}
}

func TestPredefinedPool(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	if _, err := a.getPredefinedPool("blue", false); err == nil {
		t.Fatal("Expected failure for non default addr space")
	}

	pid, nw, _, err := a.RequestPool(localAddressSpace, "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	nw2, err := a.getPredefinedPool(localAddressSpace, false)
	if err != nil {
		t.Fatal(err)
	}
	if types.CompareIPNet(nw, nw2) {
		t.Fatalf("Unexpected default network returned: %s = %s", nw2, nw)
	}

	if err := a.ReleasePool(pid); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveSubnet(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	a.addrSpaces["splane"] = &addrSpace{
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		subnets: map[SubnetKey]*PoolData{},
	}

	input := []struct {
		addrSpace string
		subnet    string
		v6        bool
	}{
		{localAddressSpace, "192.168.0.0/16", false},
		{localAddressSpace, "172.17.0.0/16", false},
		{localAddressSpace, "10.0.0.0/8", false},
		{localAddressSpace, "2001:db8:1:2:3:4:ffff::/112", false},
		{"splane", "172.17.0.0/16", false},
		{"splane", "10.0.0.0/8", false},
		{"splane", "2001:db8:1:2:3:4:5::/112", true},
		{"splane", "2001:db8:1:2:3:4:ffff::/112", true},
	}

	poolIDs := make([]string, len(input))

	for ind, i := range input {
		if poolIDs[ind], _, _, err = a.RequestPool(i.addrSpace, i.subnet, "", nil, i.v6); err != nil {
			t.Fatalf("Failed to apply input. Can't proceed: %s", err.Error())
		}
	}

	for ind, id := range poolIDs {
		if err := a.ReleasePool(id); err != nil {
			t.Fatalf("Failed to release poolID %s (%d)", id, ind)
		}
	}
}

func TestGetSameAddress(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	a.addrSpaces["giallo"] = &addrSpace{
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		subnets: map[SubnetKey]*PoolData{},
	}

	pid, _, _, err := a.RequestPool("giallo", "192.168.100.0/24", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	ip := net.ParseIP("192.168.100.250")
	_, _, err = a.RequestAddress(pid, ip, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = a.RequestAddress(pid, ip, nil)
	if err == nil {
		t.Fatal(err)
	}
}

func TestPoolAllocationReuse(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	// First get all pools until they are exhausted to
	pList := []string{}
	pool, _, _, err := a.RequestPool(localAddressSpace, "", "", nil, false)
	for err == nil {
		pList = append(pList, pool)
		pool, _, _, err = a.RequestPool(localAddressSpace, "", "", nil, false)
	}
	nPools := len(pList)
	for _, pool := range pList {
		if err := a.ReleasePool(pool); err != nil {
			t.Fatal(err)
		}
	}

	// Now try to allocate then free nPool pools sequentially.
	// Verify that we don't see any repeat networks even though
	// we have freed them.
	seen := map[string]bool{}
	for i := 0; i < nPools; i++ {
		pool, nw, _, err := a.RequestPool(localAddressSpace, "", "", nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := seen[nw.String()]; ok {
			t.Fatalf("Network %s was reused before exhausing the pool list", nw.String())
		}
		seen[nw.String()] = true
		if err := a.ReleasePool(pool); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetAddressSubPoolEqualPool(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	// Requesting a subpool of same size of the master pool should not cause any problem on ip allocation
	pid, _, _, err := a.RequestPool(localAddressSpace, "172.18.0.0/16", "172.18.0.0/16", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = a.RequestAddress(pid, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestReleaseAddressFromSubPool(t *testing.T) {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	a.addrSpaces["rosso"] = &addrSpace{
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		subnets: map[SubnetKey]*PoolData{},
	}

	poolID, _, _, err := a.RequestPool("rosso", "172.28.0.0/16", "172.28.30.0/24", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	var ip *net.IPNet
	expected := &net.IPNet{IP: net.IP{172, 28, 30, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(poolID, nil, nil); err == nil {
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
	if err = a.ReleaseAddress(poolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(poolID, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	_, _, _, err = a.RequestPool("rosso", "10.0.0.0/8", "10.0.0.0/16", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	poolID, _, _, err = a.RequestPool("rosso", "10.0.0.0/16", "10.0.0.0/24", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	expected = &net.IPNet{IP: net.IP{10, 0, 0, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(poolID, nil, nil); err == nil {
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
	if err = a.ReleaseAddress(poolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(poolID, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	// Request any addresses from subpool after explicit address request
	unoExp, _ := types.ParseCIDR("10.2.2.0/16")
	dueExp, _ := types.ParseCIDR("10.2.2.2/16")
	treExp, _ := types.ParseCIDR("10.2.2.1/16")

	if poolID, _, _, err = a.RequestPool("rosso", "10.2.0.0/16", "10.2.2.0/24", nil, false); err != nil {
		t.Fatal(err)
	}
	tre, _, err := a.RequestAddress(poolID, treExp.IP, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, treExp) {
		t.Fatalf("Unexpected address: %v", tre)
	}

	uno, _, err := a.RequestAddress(poolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, unoExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	due, _, err := a.RequestAddress(poolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(due, dueExp) {
		t.Fatalf("Unexpected address: %v", due)
	}

	if err = a.ReleaseAddress(poolID, uno.IP); err != nil {
		t.Fatal(err)
	}
	uno, _, err = a.RequestAddress(poolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, unoExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	if err = a.ReleaseAddress(poolID, tre.IP); err != nil {
		t.Fatal(err)
	}
	tre, _, err = a.RequestAddress(poolID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, treExp) {
		t.Fatalf("Unexpected address: %v", tre)
	}
}

func TestSerializeRequestReleaseAddressFromSubPool(t *testing.T) {
	opts := map[string]string{
		ipamapi.AllocSerialPrefix: "true"}
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	a.addrSpaces["rosso"] = &addrSpace{
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		subnets: map[SubnetKey]*PoolData{},
	}

	poolID, _, _, err := a.RequestPool("rosso", "172.28.0.0/16", "172.28.30.0/24", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	var ip *net.IPNet
	expected := &net.IPNet{IP: net.IP{172, 28, 30, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(poolID, nil, opts); err == nil {
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
	if err = a.ReleaseAddress(poolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(poolID, nil, opts); err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(rp, ip) {
		t.Fatalf("Unexpected IP from subpool. Expected: %s. Got: %v.", rp, ip)
	}

	_, _, _, err = a.RequestPool("rosso", "10.0.0.0/8", "10.0.0.0/16", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	poolID, _, _, err = a.RequestPool("rosso", "10.0.0.0/16", "10.0.0.0/24", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	expected = &net.IPNet{IP: net.IP{10, 0, 0, 255}, Mask: net.IPMask{255, 255, 0, 0}}
	for err == nil {
		var c *net.IPNet
		if c, _, err = a.RequestAddress(poolID, nil, opts); err == nil {
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
	if err = a.ReleaseAddress(poolID, rp.IP); err != nil {
		t.Fatal(err)
	}
	if ip, _, err = a.RequestAddress(poolID, nil, opts); err != nil {
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
	if poolID, _, _, err = a.RequestPool("rosso", "10.2.0.0/16", "10.2.2.0/24", nil, false); err != nil {
		t.Fatal(err)
	}
	tre, _, err := a.RequestAddress(poolID, treExp.IP, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(tre, treExp) {
		t.Fatalf("Unexpected address: %v", tre)
	}

	uno, _, err := a.RequestAddress(poolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, unoExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	due, _, err := a.RequestAddress(poolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(due, dueExp) {
		t.Fatalf("Unexpected address: %v", due)
	}

	if err = a.ReleaseAddress(poolID, uno.IP); err != nil {
		t.Fatal(err)
	}
	uno, _, err = a.RequestAddress(poolID, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(uno, quaExp) {
		t.Fatalf("Unexpected address: %v", uno)
	}

	if err = a.ReleaseAddress(poolID, tre.IP); err != nil {
		t.Fatal(err)
	}
	tre, _, err = a.RequestAddress(poolID, nil, opts)
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
		"10.0.0.0/29", "10.0.0.0/30", "10.0.0.0/31"}

	for _, subnet := range input {
		assertGetAddress(t, subnet)
	}
}

func TestRequestSyntaxCheck(t *testing.T) {
	var (
		pool    = "192.168.0.0/16"
		subPool = "192.168.0.0/24"
		as      = "green"
	)

	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	a.addrSpaces[as] = &addrSpace{
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		subnets: map[SubnetKey]*PoolData{},
	}

	_, _, _, err = a.RequestPool("", pool, "", nil, false)
	if err == nil {
		t.Fatal("Failed to detect wrong request: empty address space")
	}

	_, _, _, err = a.RequestPool("", pool, subPool, nil, false)
	if err == nil {
		t.Fatal("Failed to detect wrong request: empty address space")
	}

	_, _, _, err = a.RequestPool(as, "", subPool, nil, false)
	if err == nil {
		t.Fatal("Failed to detect wrong request: subPool specified and no pool")
	}

	pid, _, _, err := a.RequestPool(as, pool, subPool, nil, false)
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	_, _, err = a.RequestAddress("", nil, nil)
	if err == nil {
		t.Fatal("Failed to detect wrong request: no pool id specified")
	}

	ip := net.ParseIP("172.17.0.23")
	_, _, err = a.RequestAddress(pid, ip, nil)
	if err == nil {
		t.Fatal("Failed to detect wrong request: requested IP from different subnet")
	}

	ip = net.ParseIP("192.168.0.50")
	_, _, err = a.RequestAddress(pid, ip, nil)
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	err = a.ReleaseAddress("", ip)
	if err == nil {
		t.Fatal("Failed to detect wrong request: no pool id specified")
	}

	err = a.ReleaseAddress(pid, nil)
	if err == nil {
		t.Fatal("Failed to detect wrong request: no pool id specified")
	}

	err = a.ReleaseAddress(pid, ip)
	if err != nil {
		t.Fatalf("Unexpected failure: %v: %s, %s", err, pid, ip)
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
			_, _, _, err = a.RequestPool(localAddressSpace, env, "", nil, false)
			assert.NilError(t, err)
		}

		// Make the test allocation.
		_, _, _, err = a.RequestPool(localAddressSpace, tc.subnet, "", nil, false)
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

	pool, _, _, err := allocator.RequestPool(localAddressSpace, subnet, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Outside-the-range

	for _, outside := range outsideTheRangeAddresses {
		_, _, errx := allocator.RequestAddress(pool, net.ParseIP(outside.address), nil)
		if errx != ipamapi.ErrIPOutOfRange {
			t.Fatalf("Address %s failed to throw expected error: %s", outside.address, errx.Error())
		}
	}

	// Should get just these two IPs followed by exhaustion on the next request

	for _, expected := range expectedAddresses {
		got, _, errx := allocator.RequestAddress(pool, nil, nil)
		if errx != nil {
			t.Fatalf("Failed to obtain the address: %s", errx.Error())
		}
		expectedIP := net.ParseIP(expected.address)
		gotIP := got.IP
		if !gotIP.Equal(expectedIP) {
			t.Fatalf("Failed to obtain sequentialaddress. Expected: %s, Got: %s", expectedIP, gotIP)
		}
	}

	_, _, err = allocator.RequestAddress(pool, nil, nil)
	if err != ipamapi.ErrNoAvailableIPs {
		t.Fatal("Did not get expected error when pool is exhausted.")
	}
}

func TestRelease(t *testing.T) {
	var (
		subnet = "192.168.0.0/23"
	)

	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	assert.NilError(t, err)

	pid, _, _, err := a.RequestPool(localAddressSpace, subnet, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate all addresses
	for err != ipamapi.ErrNoAvailableIPs {
		_, _, err = a.RequestAddress(pid, nil, nil)
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
		a.ReleaseAddress(pid, ip0)
		bm := a.addresses[SubnetKey{localAddressSpace, subnet, ""}]
		if bm.Unselected() != 1 {
			t.Fatalf("Failed to update free address count after release. Expected %d, Found: %d", i+1, bm.Unselected())
		}

		nw, _, err := a.RequestAddress(pid, nil, nil)
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
		a         = &Allocator{}
	)

	_, sub, _ := net.ParseCIDR(subnet)
	ones, bits := sub.Mask.Size()
	zeroes := bits - ones
	numAddresses := 1 << uint(zeroes)

	bm, err := bitseq.NewHandle("ipam_test", nil, "default/"+subnet, uint64(numAddresses))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	run := 0
	for err != ipamapi.ErrNoAvailableIPs {
		_, err = a.getAddress(sub, bm, nil, nil, false)
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

	pid, _, _, err := a.RequestPool(localAddressSpace, subnet, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	i := 0
	start := time.Now()
	for ; i < numReq; i++ {
		nw, _, err = a.RequestAddress(pid, nil, nil)
	}
	if printTime {
		fmt.Printf("\nTaken %v, to allocate %d addresses on %s\n", time.Since(start), numReq, subnet)
	}

	if !lastIP.Equal(nw.IP) {
		t.Fatalf("Wrong last IP. Expected %s. Got: %s (err: %v, ind: %d)", lastExpectedIP, nw.IP.String(), err, i)
	}
}

func benchmarkRequest(b *testing.B, a *Allocator, subnet string) {
	pid, _, _, err := a.RequestPool(localAddressSpace, subnet, "", nil, false)
	for err != ipamapi.ErrNoAvailableIPs {
		_, _, err = a.RequestAddress(pid, nil, nil)
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

	pid, _, _, err := a.RequestPool(localAddressSpace, pool, subPool, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate num ip addresses
	indices := make(map[int]*net.IPNet, num)
	allocated := make(map[string]bool, num)
	for i := 0; i < num; i++ {
		ip, _, err := a.RequestAddress(pid, nil, nil)
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
		err := a.ReleaseAddress(pid, ip.IP)
		if err != nil {
			t.Fatalf("Unexpected failure on deallocation of %s: %v.\nSeed: %d.", ip, err, seed)
		}
		delete(indices, idx)
		delete(allocated, ip.String())
	}

	// Request a quarter of addresses
	for i := 0; i < num/2; i++ {
		ip, _, err := a.RequestAddress(pid, nil, nil)
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

	_, pools[instance], _, err = allocator.RequestPool(localAddressSpace, "", "", nil, false)
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
	a.addrSpaces["rosso"] = &addrSpace{
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		subnets: map[SubnetKey]*PoolData{},
	}

	opts := map[string]string{
		ipamapi.AllocSerialPrefix: "true",
	}
	var l sync.Mutex

	poolID, _, _, err := a.RequestPool("rosso", "198.168.0.0/23", "", nil, false)
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
			if c, _, err = a.RequestAddress(poolID, nil, opts); err == nil {
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
				return a.ReleaseAddress(poolID, ip.IP)
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
