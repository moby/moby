package rlkclient

import (
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestChildHostIP(t *testing.T) {
	builtin := &PortDriverClient{
		portDriverName: "builtin",
		protos:         map[string]struct{}{"tcp4": {}, "tcp6": {}, "udp4": {}, "udp6": {}},
	}
	slirp4netns := &PortDriverClient{
		portDriverName: "slirp4netns",
		protos:         map[string]struct{}{"tcp4": {}, "udp4": {}},
		childIP:        netip.MustParseAddr("10.0.2.100"),
	}

	testcases := []struct {
		name   string
		pdc    *PortDriverClient
		proto  string
		hostIP netip.Addr
		want   netip.Addr
	}{
		{
			name:   "nil client",
			proto:  "tcp",
			hostIP: netip.MustParseAddr("127.0.1.2"),
			want:   netip.MustParseAddr("127.0.1.2"),
		},
		{
			name:   "unsupported proto",
			pdc:    slirp4netns,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("::1"),
		},
		{
			name:   "forced child IP",
			pdc:    slirp4netns,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("127.0.1.2"),
			want:   netip.MustParseAddr("10.0.2.100"),
		},
		{
			name:   "v4 unspecified",
			pdc:    builtin,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("0.0.0.0"),
			want:   netip.MustParseAddr("127.0.0.1"),
		},
		{
			name:   "v4 non-loopback",
			pdc:    builtin,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("192.0.2.1"),
			want:   netip.MustParseAddr("127.0.0.1"),
		},
		{
			name:   "v4 default loopback",
			pdc:    builtin,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("127.0.0.1"),
			want:   netip.MustParseAddr("127.0.0.1"),
		},
		{
			// Distinct loopback addresses must not be collapsed to
			// 127.0.0.1, or bindings on the same port collide in the
			// child namespace.
			// Regression test for https://github.com/moby/moby/issues/52783
			name:   "v4 non-default loopback",
			pdc:    builtin,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("127.0.1.2"),
			want:   netip.MustParseAddr("127.0.1.2"),
		},
		{
			name:   "v6 loopback",
			pdc:    builtin,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("::1"),
			want:   netip.IPv6Loopback(),
		},
		{
			name:   "v6 non-loopback",
			pdc:    builtin,
			proto:  "tcp",
			hostIP: netip.MustParseAddr("2001:db8::1"),
			want:   netip.IPv6Loopback(),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pdc.ChildHostIP(tc.proto, tc.hostIP)
			assert.Check(t, is.Equal(got, tc.want))
		})
	}
}
