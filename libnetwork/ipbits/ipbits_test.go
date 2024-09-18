package ipbits

import (
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		in    netip.Addr
		x     uint64
		shift uint
		want  netip.Addr
	}{
		{netip.MustParseAddr("10.0.0.1"), 0, 0, netip.MustParseAddr("10.0.0.1")},
		{netip.MustParseAddr("10.0.0.1"), 41, 0, netip.MustParseAddr("10.0.0.42")},
		{netip.MustParseAddr("10.0.0.1"), 42, 16, netip.MustParseAddr("10.42.0.1")},
		{netip.MustParseAddr("10.0.0.1"), 1, 7, netip.MustParseAddr("10.0.0.129")},
		{netip.MustParseAddr("10.0.0.1"), 1, 24, netip.MustParseAddr("11.0.0.1")},
		{netip.MustParseAddr("2001::1"), 0, 0, netip.MustParseAddr("2001::1")},
		{netip.MustParseAddr("2001::1"), 0x41, 0, netip.MustParseAddr("2001::42")},
		{netip.MustParseAddr("2001::1"), 1, 7, netip.MustParseAddr("2001::81")},
		{netip.MustParseAddr("2001::1"), 0xcafe, 96, netip.MustParseAddr("2001:cafe::1")},
		{netip.MustParseAddr("2001::1"), 1, 112, netip.MustParseAddr("2002::1")},
	}

	for _, tt := range tests {
		if got := Add(tt.in, tt.x, tt.shift); tt.want != got {
			t.Errorf("%v + (%v << %v) = %v; want %v", tt.in, tt.x, tt.shift, got, tt.want)
		}
	}
}

func BenchmarkAdd(b *testing.B) {
	do := func(b *testing.B, addr netip.Addr) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = Add(addr, uint64(i), 0)
		}
	}

	b.Run("IPv4", func(b *testing.B) { do(b, netip.IPv4Unspecified()) })
	b.Run("IPv6", func(b *testing.B) { do(b, netip.IPv6Unspecified()) })
}

func TestField(t *testing.T) {
	tests := []struct {
		in   netip.Addr
		u, v uint
		want uint64
	}{
		{netip.MustParseAddr("1.2.3.4"), 0, 8, 1},
		{netip.MustParseAddr("1.2.3.4"), 8, 16, 2},
		{netip.MustParseAddr("1.2.3.4"), 16, 24, 3},
		{netip.MustParseAddr("1.2.3.4"), 24, 32, 4},
		{netip.MustParseAddr("1.2.3.4"), 0, 32, 0x01020304},
		{netip.MustParseAddr("1.2.3.4"), 0, 28, 0x102030},
		{netip.MustParseAddr("1234:5678:9abc:def0::7654:3210"), 0, 8, 0x12},
		{netip.MustParseAddr("1234:5678:9abc:def0::7654:3210"), 8, 16, 0x34},
		{netip.MustParseAddr("1234:5678:9abc:def0::7654:3210"), 16, 24, 0x56},
		{netip.MustParseAddr("1234:5678:9abc:def0::7654:3210"), 64, 128, 0x76543210},
		{netip.MustParseAddr("1234:5678:9abc:def0:beef::7654:3210"), 48, 80, 0xdef0beef},
	}

	for _, tt := range tests {
		if got := Field(tt.in, tt.u, tt.v); got != tt.want {
			t.Errorf("Field(%v, %v, %v) = %v (0x%[4]x); want %v (0x%[5]x)", tt.in, tt.u, tt.v, got, tt.want)
		}
	}
}

func TestSubnetsBetween(t *testing.T) {
	tests := []struct {
		a1, a2 netip.Addr
		sz     int
		want   uint64
	}{
		{netip.MustParseAddr("10.0.0.0"), netip.MustParseAddr("10.0.0.0"), 8, 0},
		{netip.MustParseAddr("10.0.0.0"), netip.MustParseAddr("10.0.10.0"), 8, 0},
		{netip.MustParseAddr("10.0.0.0"), netip.MustParseAddr("10.1.0.0"), 24, 256},
		{netip.MustParseAddr("10.0.0.0"), netip.MustParseAddr("10.10.0.0"), 16, 10},
		{netip.MustParseAddr("10.20.0.0"), netip.MustParseAddr("10.20.128.0"), 24, 128},
		{netip.MustParseAddr("10.0.0.0"), netip.MustParseAddr("10.0.10.0"), 24, 10},

		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fc00::"), 8, 0x0},
		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fc00:1000::"), 16, 0x0},
		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fc01::"), 24, 0x100},
		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fc01::"), 16, 0x1},
		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fc00:1000::"), 24, 0x10},
		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fc00:1000::"), 24, 0x10},
		{netip.MustParseAddr("fc00::"), netip.MustParseAddr("fd00::"), 64, 0x100_0000_0000_0000},
	}

	for _, tt := range tests {
		d := SubnetsBetween(tt.a1, tt.a2, tt.sz)
		assert.Check(t, d == tt.want, "SubnetsBetween(%q, %q, %d) = 0x%x; want: 0x%x", tt.a1, tt.a2, tt.sz, d, tt.want)
	}
}
