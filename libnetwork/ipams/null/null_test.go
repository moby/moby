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
	assert.Check(t, is.Equal(alloc.PoolID, defaultPoolID))
	assert.Check(t, is.Equal(alloc.Pool, defaultPool))

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: "default"})
	assert.ErrorContains(t, err, "unknown address space: default")

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace, Pool: "192.168.0.0/16"})
	assert.ErrorContains(t, err, "null ipam driver does not handle specific address pool requests")

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace, SubPool: "192.168.0.0/24"})
	assert.ErrorContains(t, err, "null ipam driver does not handle specific address subpool requests")

	_, err = a.RequestPool(ipamapi.PoolRequest{AddressSpace: defaultAddressSpace, V6: true})
	assert.ErrorContains(t, err, "null ipam driver does not handle IPv6 address pool requests")
}

func TestOtherRequests(t *testing.T) {
	a := allocator{}

	ip, _, err := a.RequestAddress(defaultPoolID, nil, nil)
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
