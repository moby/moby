package netiputil

import (
	"net"
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLastAddr(t *testing.T) {
	testcases := []struct {
		p    netip.Prefix
		want netip.Addr
	}{
		{netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("10.0.0.255")},
		{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParseAddr("10.255.255.255")},
		{netip.MustParsePrefix("fd00::/64"), netip.MustParseAddr("fd00::ffff:ffff:ffff:ffff")},
		{netip.MustParsePrefix("fd00::/16"), netip.MustParseAddr("fd00:ffff:ffff:ffff:ffff:ffff:ffff:ffff")},
		{netip.MustParsePrefix("ffff::/16"), netip.MustParseAddr("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")},
	}

	for _, tc := range testcases {
		last := LastAddr(tc.p)
		assert.Check(t, last == tc.want, "LastAddr(%q) = %s; want: %s", tc.p, last, tc.want)
	}
}

func TestPrefixAfter(t *testing.T) {
	testcases := []struct {
		prev netip.Prefix
		sz   int
		want netip.Prefix
	}{
		{netip.MustParsePrefix("10.0.10.0/24"), 24, netip.MustParsePrefix("10.0.11.0/24")},
		{netip.MustParsePrefix("10.0.10.0/24"), 16, netip.MustParsePrefix("10.1.0.0/16")},
		{netip.MustParsePrefix("10.10.0.0/16"), 24, netip.MustParsePrefix("10.11.0.0/24")},
		{netip.MustParsePrefix("2001:db8:feed:cafe:b000:dead::/96"), 16, netip.MustParsePrefix("2002::/16")},
		{netip.MustParsePrefix("ffff::/16"), 16, netip.Prefix{}},
		{netip.MustParsePrefix("2001:db8:1::/48"), 64, netip.MustParsePrefix("2001:db8:2::/64")},
	}

	for _, tc := range testcases {
		next := PrefixAfter(tc.prev, tc.sz)
		assert.Check(t, next == tc.want, "PrefixAfter(%q, %d) = %s; want: %s", tc.prev, tc.sz, next, tc.want)
	}
}

func TestUnmap(t *testing.T) {
	assert.Check(t, !Unmap(netip.Prefix{}).IsValid())
}

func TestParseCIDR(t *testing.T) {
	tests := []string{"::ffff:1.2.3.4/24", "::ffff:1.2.3.4/1", "::ffff:1.2.3.4/120", "1.2.3.4/24", "::1/128", "2001:db8::/32"}
	for _, s := range tests {
		// From "github.com/moby/moby/v2/daemon/libnetwork/types".ParseCIDR
		ip, net, err := net.ParseCIDR(s)
		assert.NilError(t, err)
		net.IP = ip
		want := netip.MustParsePrefix(net.String())

		got, err := ParseCIDR(s)
		assert.Check(t, err)
		if got != want {
			t.Errorf("ParseCIDR(%q) = %v, want %v", s, got, want)
		}
	}

	got, err := ParseCIDR("invalid")
	assert.Check(t, err != nil, "expected error for invalid input")
	assert.Check(t, !got.IsValid(), "expected invalid result for invalid input")
}

func TestMaybeParse(t *testing.T) {
	addr := MaybeParse(netip.ParseAddr)
	got, err := addr("")
	assert.Check(t, err)
	assert.Check(t, !got.IsValid())

	got, err = addr("bogus")
	assert.Check(t, err != nil)
	assert.Check(t, !got.IsValid())

	got, err = addr("1.2.3.4")
	if assert.Check(t, err) {
		assert.Check(t, is.Equal(got, netip.MustParseAddr("1.2.3.4")))
	}
}
