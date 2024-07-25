package null

import (
	"testing"

	"github.com/docker/docker/libnetwork/ipamapi"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPoolRequest(t *testing.T) {
	a := allocator{}

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, defaultPoolID4))
	assert.Check(t, is.Equal(alloc.Pool, defaultPool4))

	alloc, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace, V6: true})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, defaultPoolID6))
	assert.Check(t, is.Equal(alloc.Pool, defaultPool6))

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: "default"})
	assert.ErrorContains(t, err, "unknown address space: default")

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace, Pool: "192.168.0.0/16"})
	assert.ErrorContains(t, err, "null ipam driver does not handle specific address pool requests")

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace, SubPool: "192.168.0.0/24"})
	assert.ErrorContains(t, err, "null ipam driver does not handle specific address subpool requests")
}

func TestOtherRequests(t *testing.T) {
	a := allocator{}

	ip, _, err := a.RequestAddress(defaultPoolID4, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ip != nil {
		t.Fatalf("Unexpected address returned: %v", ip)
	}

	ip, _, err = a.RequestAddress(defaultPoolID6, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ip != nil {
		t.Fatalf("Unexpected address returned: %v", ip)
	}

	_, _, err = a.RequestAddress("anypid", nil, nil)
	if err == nil {
		t.Fatal("Unexpected success")
	}
}
