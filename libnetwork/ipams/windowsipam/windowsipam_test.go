//go:build windows

package windowsipam

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWindowsIPAM(t *testing.T) {
	a := &allocator{}

	alloc, err := a.RequestPool(ipamapi.PoolRequest{
		AddressSpace: localAddressSpace,
		Pool:         "192.168.0.0/16",
	})
	assert.NilError(t, err)

	requestAddress := net.ParseIP("192.168.1.1")

	ip, _, err := a.RequestAddress(alloc.PoolID, nil, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	if ip != nil {
		t.Fatalf("Unexpected data returned. Expected %v . Got: %v ", alloc.PoolID, ip)
	}

	ip, _, err = a.RequestAddress(alloc.PoolID, requestAddress, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	if !ip.IP.Equal(requestAddress) {
		t.Fatalf("Unexpected data returned. Expected %v . Got: %v ", requestAddress, ip.IP)
	}

	requestOptions := map[string]string{}
	requestOptions[ipamapi.RequestAddressType] = netlabel.Gateway
	ip, _, err = a.RequestAddress(alloc.PoolID, requestAddress, requestOptions)
	if err != nil {
		t.Fatal(err)
	}

	if !ip.IP.Equal(requestAddress) {
		t.Fatalf("Unexpected data returned. Expected %v . Got: %v ", requestAddress, ip.IP)
	}

	err = a.ReleaseAddress(alloc.PoolID, requestAddress)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestPool(t *testing.T) {
	testcases := []struct {
		req      ipamapi.PoolRequest
		expAlloc ipamapi.AllocatedPool
		expErr   error
	}{
		{
			req: ipamapi.PoolRequest{AddressSpace: localAddressSpace},
			expAlloc: ipamapi.AllocatedPool{
				PoolID: defaultPool.String(),
				Pool:   defaultPool,
			},
		},
		{
			req: ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "192.168.0.0/16"},
			expAlloc: ipamapi.AllocatedPool{
				PoolID: "192.168.0.0/16",
				Pool:   netip.MustParsePrefix("192.168.0.0/16"),
			},
		},
		{
			req:    ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: "192.168.0.0/16", SubPool: "192.168.0.0/16"},
			expErr: errors.New("this request is not supported by the 'windows' ipam driver"),
		},
		{
			req:    ipamapi.PoolRequest{AddressSpace: localAddressSpace, V6: true},
			expErr: errors.New("this request is not supported by the 'windows' ipam driver"),
		},
	}

	a := &allocator{}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%+v", tc.req), func(t *testing.T) {
			alloc, err := a.RequestPool(tc.req)

			if tc.expErr != nil {
				assert.Error(t, err, tc.expErr.Error())
			} else {
				assert.NilError(t, err)
			}

			assert.DeepEqual(t, alloc.Pool, tc.expAlloc.Pool, cmpopts.EquateComparable(netip.Prefix{}))
			assert.Equal(t, alloc.PoolID, tc.expAlloc.PoolID)
		})
	}
}

func TestReleasePool(t *testing.T) {
	a := &allocator{}

	alloc, err := a.RequestPool(ipamapi.PoolRequest{
		AddressSpace: localAddressSpace,
		Pool:         "192.168.0.0/16",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, "192.168.0.0/16"))
	assert.Check(t, is.Equal(alloc.Pool.String(), "192.168.0.0/16"))

	err = a.ReleasePool(alloc.PoolID)
	assert.NilError(t, err)
}
