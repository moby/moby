package defaultipam

import (
	"net/netip"
	"testing"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNewAddrSpaceDedup(t *testing.T) {
	as, err := newAddrSpace([]*ipamutils.NetworkToSplit{
		{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 16},
		{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 24},
		{Base: netip.MustParsePrefix("10.10.0.0/8"), Size: 24},
		{Base: netip.MustParsePrefix("10.0.100.0/8"), Size: 24},
		{Base: netip.MustParsePrefix("192.168.0.0/24"), Size: 24},
	})
	assert.NilError(t, err)

	assert.DeepEqual(t, as.predefined, []*ipamutils.NetworkToSplit{
		{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 16},
		{Base: netip.MustParsePrefix("192.168.0.0/24"), Size: 24},
	}, cmpopts.EquateComparable(ipamutils.NetworkToSplit{}))
}

func TestDynamicPoolAllocation(t *testing.T) {
	testcases := []struct {
		name       string
		predefined []*ipamutils.NetworkToSplit
		allocated  []netip.Prefix
		reserved   []netip.Prefix
		expPrefix  netip.Prefix
		expErr     error
	}{
		{
			name: "First allocated overlaps at the end of first pool",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("192.168.255.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "First reserved bigger than first allocated",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("10.0.0.0/8"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.0/7"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "First pool fully overlapped by bigger allocated, next overlapped in the middle",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.20.0.0/16"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.0/8"),
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("192.168.128.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "First pool fully overlapped by bigger allocated, next overlapped at the beginning and in the middle",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.20.0.0/16"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.0/8"),
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("192.168.0.0/24"),
				netip.MustParsePrefix("192.168.128.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.1.0/24"),
		},
		{
			name: "First pool fully overlapped by smaller prefixes, next overlapped in the middle",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.20.0.0/22"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("10.20.0.0/24"),
				netip.MustParsePrefix("10.20.1.0/24"),
				netip.MustParsePrefix("10.20.2.0/24"),
				netip.MustParsePrefix("192.168.128.0/24"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("10.20.3.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "First pool fully overlapped by smaller prefix, next predefined before reserved",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.20.0.0/16"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("10.20.0.0/17"),
				netip.MustParsePrefix("10.20.128.0/17"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("200.1.2.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "First pool fully overlapped by smaller prefix, reserved is the same as the last allocated subnet",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.10.0.0/22"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("10.10.0.0/24"),
				netip.MustParsePrefix("10.10.1.0/24"),
				netip.MustParsePrefix("10.10.2.0/24"),
				netip.MustParsePrefix("10.10.3.0/24"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("10.10.3.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "Partial overlap by allocated of different sizes",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("192.168.0.0/24"),
				netip.MustParsePrefix("192.168.1.0/24"),
				netip.MustParsePrefix("192.168.2.0/23"),
				netip.MustParsePrefix("192.168.4.3/30"),
			},
			expPrefix: netip.MustParsePrefix("192.168.5.0/24"),
		},
		{
			name: "Partial overlap at the start, not enough space left",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.0.0.0/31"), Size: 31},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				// Partial overlap, not enough space left
				netip.MustParsePrefix("10.0.0.0/32"),
				netip.MustParsePrefix("100.0.0.0/32"),
				netip.MustParsePrefix("200.0.0.0/32"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "Partial overlap by allocations and reserved of different sizes",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("192.168.0.0/24"),
				netip.MustParsePrefix("192.168.1.0/24"),
				netip.MustParsePrefix("192.168.2.3/30"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("192.168.2.4/30"),
				netip.MustParsePrefix("192.168.3.0/30"),
				netip.MustParsePrefix("192.168.4.0/23"),
			},
			expPrefix: netip.MustParsePrefix("192.168.6.0/24"),
		},
		{
			name: "Partial overlap, same prefix in allocated and reserved",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				// Partial overlap with enough space remaining
				netip.MustParsePrefix("192.168.0.0/24"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("192.168.0.0/24"),
			},
			expPrefix: netip.MustParsePrefix("192.168.1.0/24"),
		},
		{
			name: "Partial overlap, two predefined",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.0/24"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("192.168.0.0/24"),
			},
			expPrefix: netip.MustParsePrefix("10.0.1.0/24"),
		},
		{
			name: "Predefined with overlapping prefixes, longer prefixes discarded",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 24},
				// This predefined will be discarded.
				{Base: netip.MustParsePrefix("10.0.0.0/16"), Size: 24},
				// This predefined will be discarded.
				{Base: netip.MustParsePrefix("10.10.0.0/16"), Size: 24},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			reserved:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "Partial overlap at the beginning, single predefined",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
			},
			expPrefix: netip.MustParsePrefix("172.17.0.0/16"),
		},
		{
			name: "Partial overlap, no space left at the end, next pool not subnetted yet",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
				{Base: netip.MustParsePrefix("192.168.0.0/16"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.0.0/17"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "Partial overlap, no space left at the end, no more predefined",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.0.0/17"),
			},
			expErr: ipamapi.ErrNoMoreSubnets,
		},
		{
			name: "Extra allocated, no pool left",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.0.0/16"),
				netip.MustParsePrefix("192.168.0.0/24"),
			},
			expErr: ipamapi.ErrNoMoreSubnets,
		},
		{
			name: "Extra reserved, no pool left",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.0.0/16"),
			},
			reserved: []netip.Prefix{
				netip.MustParsePrefix("192.168.0.0/24"),
			},
			expErr: ipamapi.ErrNoMoreSubnets,
		},
		{
			name: "Predefined fully allocated",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
				{Base: netip.MustParsePrefix("192.168.0.0/23"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.0.0/16"),
				netip.MustParsePrefix("192.168.0.0/24"),
				netip.MustParsePrefix("192.168.1.0/24"),
			},
			expErr: ipamapi.ErrNoMoreSubnets,
		},
		{
			name: "Partial overlap, not enough space left",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
				{Base: netip.MustParsePrefix("192.168.0.0/23"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.128.0/17"),
				netip.MustParsePrefix("192.168.0.1/32"),
				netip.MustParsePrefix("192.168.1.0/24"),
			},
			expErr: ipamapi.ErrNoMoreSubnets,
		},
		{
			name: "Duplicate 'allocated' at the end of a predefined",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
				{Base: netip.MustParsePrefix("192.168.0.0/23"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.17.128.0/17"),
				netip.MustParsePrefix("172.17.128.0/17"),
				netip.MustParsePrefix("172.17.128.0/17"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
		{
			name: "Duplicate 'allocated'",
			predefined: []*ipamutils.NetworkToSplit{
				{Base: netip.MustParsePrefix("172.16.0.0/15"), Size: 16},
				{Base: netip.MustParsePrefix("192.168.0.0/23"), Size: 24},
			},
			allocated: []netip.Prefix{
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("172.16.120.0/24"),
				netip.MustParsePrefix("172.17.128.0/17"),
				netip.MustParsePrefix("172.17.128.0/17"),
				netip.MustParsePrefix("172.17.128.0/24"),
				netip.MustParsePrefix("172.17.128.0/17"),
			},
			expPrefix: netip.MustParsePrefix("192.168.0.0/24"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			as, err := newAddrSpace(tc.predefined)
			assert.NilError(t, err)
			as.allocated = tc.allocated

			p, err := as.allocatePredefinedPool(tc.reserved)

			assert.Check(t, is.ErrorIs(err, tc.expErr))
			assert.Check(t, is.Equal(p, tc.expPrefix))
		})
	}
}

func TestStaticAllocation(t *testing.T) {
	as, err := newAddrSpace([]*ipamutils.NetworkToSplit{
		{Base: netip.MustParsePrefix("10.0.0.0/8"), Size: 24},
	})
	assert.NilError(t, err)

	for _, alloc := range []netip.Prefix{
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.1.0.0/16"),
		netip.MustParsePrefix("10.0.0.0/31"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.3.0/24"),
	} {
		err := as.allocatePool(alloc)
		assert.NilError(t, err)
	}

	assert.Check(t, is.DeepEqual(as.allocated, []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.0.0/31"),
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.1.0.0/16"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("192.168.3.0/24"),
	}, cmpopts.EquateComparable(netip.Prefix{})))
}
