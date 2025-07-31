package addrset

import (
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIPv4Pool(t *testing.T) {
	var (
		// It shouldn't matter that host bits are set in the pool Prefix.
		subnet = netip.MustParsePrefix("10.20.30.40/16")
		as     = New(subnet)
		addr   netip.Addr
		err    error
	)

	assert.Check(t, is.Len(as.bitmaps, 0))

	// Add the first and last addresses in the range.
	// Expect a single bitmap of 65536 bits, with two bits set.
	err = as.Add(netip.MustParseAddr("10.20.0.0"))
	assert.Assert(t, err)
	err = as.Add(netip.MustParseAddr("10.20.255.255"))
	assert.Assert(t, err)
	assert.Check(t, is.Len(as.bitmaps, 1))
	bm := as.bitmaps[subnet.Masked()]
	assert.Check(t, is.Equal(bm.Bits(), uint64(65536)))
	assert.Check(t, is.Equal(bm.Unselected(), uint64(65534)))
	assert.Check(t, is.Equal(as.Selected(), uint64(2)))

	// Add an address that's already present. Expect an error.
	err = as.Add(netip.MustParseAddr("10.20.255.255"))
	assert.Check(t, is.ErrorIs(err, ErrAllocated))

	// Remove an address that isn't in the set. Expect no error.
	err = as.Remove(netip.MustParseAddr("10.20.30.40"))
	assert.Check(t, err)

	// Remove all addresses, expect to end up with no bitmap.
	err = as.Remove(netip.MustParseAddr("10.20.0.0"))
	assert.Check(t, err)
	err = as.Remove(netip.MustParseAddr("10.20.255.255"))
	assert.Check(t, err)
	assert.Check(t, is.Len(as.bitmaps, 0))
	assert.Check(t, is.Equal(as.Selected(), uint64(0)))

	// Remove an address that isn't in the set (now there's no bitmap). Expect no error.
	err = as.Remove(netip.MustParseAddr("10.20.30.40"))
	assert.Check(t, err)

	// Add any two addresses to the set, with serial=true. Expect the first two addresses.
	addr, err = as.AddAny(true)
	assert.Check(t, err)
	assert.Check(t, is.Equal(addr, netip.MustParseAddr("10.20.0.0")))
	addr, err = as.AddAny(true)
	assert.Check(t, err)
	assert.Check(t, is.Equal(addr, netip.MustParseAddr("10.20.0.1")))
	assert.Check(t, is.Equal(as.Selected(), uint64(2)))

	// Add any address in a range. It shouldn't matter that host bits are set in the
	// range. Expect the first address in that range.
	addr, err = as.AddAnyInRange(netip.MustParsePrefix("10.20.30.40/24"), true)
	assert.Check(t, err)
	assert.Check(t, is.Equal(addr, netip.MustParseAddr("10.20.30.0")))
}

func TestIPv6Pool(t *testing.T) {
	var (
		subnet = netip.MustParsePrefix("fddd::dddd/56")
		as     = New(subnet)
		addr   netip.Addr
		err    error
	)

	assert.Check(t, is.Len(as.bitmaps, 0))

	// Add the first and last addresses in the range.
	// Expect two bitmaps of 1<<maxBitsPerBitmap bits, with one bit set in each.
	err = as.Add(netip.MustParseAddr("fddd::"))
	assert.Assert(t, err)
	err = as.Add(netip.MustParseAddr("fddd::ff:ffff:ffff:ffff:ffff"))
	assert.Assert(t, err)
	assert.Check(t, is.Len(as.bitmaps, 2))
	for _, bm := range as.bitmaps {
		assert.Check(t, is.Equal(bm.Bits(), uint64(1)<<maxBitsPerBitmap))
		assert.Check(t, is.Equal(bm.Unselected(), (uint64(1)<<maxBitsPerBitmap)-1))
	}

	// Add an address that's already present in the "upper" bitmap. Expect an error.
	err = as.Add(netip.MustParseAddr("fddd::ff:ffff:ffff:ffff:ffff"))
	assert.Check(t, is.ErrorIs(err, ErrAllocated))
	assert.Check(t, is.Equal(as.Selected(), uint64(2)))

	// Remove an address that isn't in the set. Expect no error.
	err = as.Remove(netip.MustParseAddr("fddd::f:0:0:0:0"))
	assert.Check(t, err)

	// Remove all addresses, expect to end up with no bitmap.
	err = as.Remove(netip.MustParseAddr("fddd::"))
	assert.Check(t, err)
	err = as.Remove(netip.MustParseAddr("fddd::ff:ffff:ffff:ffff:ffff"))
	assert.Check(t, err)
	assert.Check(t, is.Len(as.bitmaps, 0))
	assert.Check(t, is.Equal(as.Selected(), uint64(0)))

	// Remove an address that isn't in the set (now there's no bitmap). Expect no error.
	err = as.Remove(netip.MustParseAddr("fddd::f:0:0:0:0"))
	assert.Check(t, err)

	// Add any two addresses to the set, with serial=true. Expect the first two addresses.
	addr, err = as.AddAny(true)
	assert.Check(t, err)
	assert.Check(t, is.Equal(addr, netip.MustParseAddr("fddd::0")))
	addr, err = as.AddAny(true)
	assert.Check(t, err)
	assert.Check(t, is.Equal(addr, netip.MustParseAddr("fddd::1")))
	assert.Check(t, is.Equal(as.Selected(), uint64(2)))

	// Add any address in a range, somewhere in the middle of the pool. Expect the first address in that range.
	addr, err = as.AddAnyInRange(netip.MustParsePrefix("fddd:0:0:f0::/60"), true)
	assert.Check(t, err)
	assert.Check(t, is.Equal(addr, netip.MustParseAddr("fddd:0:0:f0::")))

	selected, err := as.CalculateSelected()
	assert.ErrorContains(t, err, "calculation of selected IPs in range doesn't support IPv6")
	assert.Check(t, is.Equal(selected, uint64(0)))
}

func Test64BitIPv6Range(t *testing.T) {
	as := New(netip.MustParsePrefix("fd75:7f12:d221:7b32::/64"))
	addr := netip.MustParseAddr("fd75:7f12:d221:7b32:94b0:97ff:fefe:52da")

	err := as.Add(addr)
	assert.Check(t, err)
	err = as.Add(addr)
	assert.Check(t, is.ErrorIs(err, ErrAllocated))
	assert.Check(t, is.Error(err, "setting bit 1490858602410169050 for fd75:7f12:d221:7b32:94b0:97ff:fefe:52da in pool 'fd75:7f12:d221:7b32::/64': address already allocated"))
	err = as.Remove(addr)
	assert.Check(t, err)
	err = as.Add(addr)
	assert.Check(t, err)
}

func Test32BitIPv6Range(t *testing.T) {
	as := New(netip.MustParsePrefix("fd75:7f12:d221:7b32::/96"))
	addr := netip.MustParseAddr("fd75:7f12:d221:7b32::fefe:52da")

	err := as.Add(addr)
	assert.Check(t, err)
	err = as.Add(addr)
	assert.Check(t, is.ErrorIs(err, ErrAllocated))
	assert.Check(t, is.Error(err, "setting bit 4278080218 for fd75:7f12:d221:7b32::fefe:52da in pool 'fd75:7f12:d221:7b32::/96': address already allocated"))
	err = as.Remove(addr)
	assert.Check(t, err)
	err = as.Add(addr)
	assert.Check(t, err)
}

func TestFullPool(t *testing.T) {
	var (
		subnet = netip.MustParsePrefix("10.20.30.0/24")
		as     = New(subnet)
		err    error
	)

	for range 256 {
		_, err = as.AddAny(true)
		assert.Check(t, err)
	}
	assert.Check(t, is.Equal(as.Selected(), uint64(256)))
	_, err = as.AddAny(true)
	assert.Check(t, is.Equal(as.Selected(), uint64(256)))
	assert.Check(t, is.ErrorIs(err, ErrNotAvailable))
	assert.Check(t, is.Error(err, "add-any to '10.20.30.0': address not available"))
}

func TestNotInPool(t *testing.T) {
	var (
		subnet = netip.MustParsePrefix("10.20.30.40/16")
		as     = New(subnet)
		addr   netip.Addr
		err    error
	)

	// Address not in pool.
	err = as.Add(netip.MustParseAddr("10.21.0.0"))
	assert.Check(t, is.Error(err, "cannot add 10.21.0.0 to '10.20.0.0/16'"))

	// Range bigger than pool.
	addr, err = as.AddAnyInRange(netip.MustParsePrefix("10.20.0.0/15"), true)
	assert.Check(t, is.Error(err, "add-any, range '10.20.0.0/15' is not in subnet '10.20.0.0/16'"))
	assert.Check(t, is.Equal(addr, netip.Addr{}))

	// Range outside pool.
	addr, err = as.AddAnyInRange(netip.MustParsePrefix("10.21.0.0/24"), true)
	assert.Check(t, is.Error(err, "add-any, range '10.21.0.0/24' is not in subnet '10.20.0.0/16'"))
	assert.Check(t, is.Equal(addr, netip.Addr{}))

	selected, err := as.CalculateSelectedInRange(netip.MustParsePrefix("10.21.0.0/24"))
	assert.ErrorContains(t, err, "subnet '10.20.0.0/16' doesn't contain '10.21.0.0/24' ip-range")
	assert.Check(t, is.Equal(selected, uint64(0)))
}

func TestInvalidPool(t *testing.T) {
	var (
		as   = New(netip.Prefix{})
		addr netip.Addr
		err  error
	)

	err = as.Add(netip.IPv6Loopback())
	assert.Check(t, is.Error(err, "cannot add ::1 to 'invalid Prefix'"))

	addr, err = as.AddAny(false)
	assert.Check(t, is.Error(err, "no bitmap to add-any to 'invalid IP': negative Prefix bits"))
	assert.Check(t, is.Equal(addr, netip.Addr{}))

	addr, err = as.AddAnyInRange(netip.Prefix{}, false)
	assert.Check(t, is.Error(err, "add-any, range 'invalid Prefix' is not in subnet 'invalid Prefix'"))
	assert.Check(t, is.Equal(addr, netip.Addr{}))
	addr, err = as.AddAnyInRange(netip.MustParsePrefix("10.20.30.0/24"), false)
	assert.Check(t, is.Error(err, "add-any, range '10.20.30.0/24' is not in subnet 'invalid Prefix'"))
	assert.Check(t, is.Equal(addr, netip.Addr{}))

	err = as.Remove(netip.MustParseAddr("10.20.30.0"))
	assert.Check(t, is.Error(err, "10.20.30.0 cannot be removed from 'invalid Prefix'"))
}

func TestSelectedCounting(t *testing.T) {
	// Test with a /29 subnet (8 addresses)
	subnet := netip.MustParsePrefix("192.168.0.0/29")
	as := New(subnet)

	// Calculate selected in empty network
	cSelected, err := as.CalculateSelected()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(0)))

	// Add a few addresses
	err = as.Add(netip.MustParseAddr("192.168.0.1"))
	assert.Assert(t, err)
	err = as.Add(netip.MustParseAddr("192.168.0.3"))
	assert.Assert(t, err)
	err = as.Add(netip.MustParseAddr("192.168.0.7"))
	assert.Assert(t, err)

	// Check bitmap and selected count
	bm := as.bitmaps[subnet.Masked()]
	assert.Check(t, is.Equal(bm.Bits(), uint64(8)))
	assert.Check(t, is.Equal(bm.Unselected(), uint64(5)))
	assert.Check(t, is.Equal(as.Selected(), uint64(3)))

	// Calculate selected in full range
	cSelected, err = as.CalculateSelected()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(3)))

	// Calculate selected in a /30 (first half)
	cSelected, err = as.CalculateSelectedInRange(netip.MustParsePrefix("192.168.0.0/30"))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(2))) // 0.1 and 0.3

	// Calculate selected in a /30 (second half)
	cSelected, err = as.CalculateSelectedInRange(netip.MustParsePrefix("192.168.0.4/30"))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(1))) // 0.7

	// Calculate selected in a /32 (single address)
	cSelected, err = as.CalculateSelectedInRange(netip.MustParsePrefix("192.168.0.3/32"))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(1))) // 0.3

	// Remove one and check again
	err = as.Remove(netip.MustParseAddr("192.168.0.3"))
	assert.Check(t, err)
	cSelected, err = as.CalculateSelected()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(2)))

	// Add more addresses to fill the pool
	_, err = as.AddAny(true)
	assert.NilError(t, err)
	_, err = as.AddAny(true)
	assert.NilError(t, err)
	_, err = as.AddAny(true)
	assert.NilError(t, err)
	_, err = as.AddAny(true)
	assert.NilError(t, err)
	_, err = as.AddAny(true)
	assert.NilError(t, err)
	_, err = as.AddAny(true)
	assert.NilError(t, err)
	_, err = as.AddAny(true)
	assert.Check(t, is.ErrorContains(err, "address not available"))

	// Calculate selected in full network
	assert.Check(t, is.Equal(as.Selected(), uint64(8)))
	cSelected, err = as.CalculateSelected()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(8)))

	// Calculate selected in a /30 (second half)
	cSelected, err = as.CalculateSelectedInRange(netip.MustParsePrefix("192.168.0.4/30"))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cSelected, uint64(4)))
}
