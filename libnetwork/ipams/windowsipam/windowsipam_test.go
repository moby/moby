//go:build windows

package windowsipam

import (
	"net"
	"testing"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWindowsIPAM(t *testing.T) {
	a := &allocator{}

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, defaultPool.String()))
	assert.Check(t, is.Equal(alloc.Pool, defaultPool))

	alloc, err = a.RequestPool(ipamapi.PoolRequest{
		AddressSpace: localAddressSpace,
		Pool:         "192.168.0.0/16",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, "192.168.0.0/16"))
	assert.Check(t, is.Equal(alloc.Pool.String(), "192.168.0.0/16"))

	_, err = a.RequestPool(ipamapi.PoolRequest{
		AddressSpace: localAddressSpace,
		Pool:         "192.168.0.0/16",
		SubPool:      "192.168.0.0/16",
	})
	assert.ErrorContains(t, err, "this request is not supported by the 'windows' ipam driver")

	_, err = a.RequestPool(ipamapi.PoolRequest{
		AddressSpace: localAddressSpace,
		V6:           true,
	})
	assert.ErrorContains(t, err, "this request is not supported by the 'windows' ipam driver")

	requestPool, _ := types.ParseCIDR("192.168.0.0/16")
	requestAddress := net.ParseIP("192.168.1.1")

	err = a.ReleasePool(requestPool.String())
	if err != nil {
		t.Fatal(err)
	}

	ip, _, err := a.RequestAddress(requestPool.String(), nil, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	if ip != nil {
		t.Fatalf("Unexpected data returned. Expected %v . Got: %v ", requestPool, ip)
	}

	ip, _, err = a.RequestAddress(requestPool.String(), requestAddress, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	if !ip.IP.Equal(requestAddress) {
		t.Fatalf("Unexpected data returned. Expected %v . Got: %v ", requestAddress, ip.IP)
	}

	requestOptions := map[string]string{}
	requestOptions[ipamapi.RequestAddressType] = netlabel.Gateway
	ip, _, err = a.RequestAddress(requestPool.String(), requestAddress, requestOptions)
	if err != nil {
		t.Fatal(err)
	}

	if !ip.IP.Equal(requestAddress) {
		t.Fatalf("Unexpected data returned. Expected %v . Got: %v ", requestAddress, ip.IP)
	}

	err = a.ReleaseAddress(requestPool.String(), requestAddress)
	if err != nil {
		t.Fatal(err)
	}
}
