package network

import (
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type subnetStub struct {
	static   bool
	contains map[string]bool
}

func (stub subnetStub) IsStatic() bool {
	return stub.static
}

func (stub subnetStub) Contains(addr netip.Addr) bool {
	v, ok := stub.contains[addr.String()]
	return ok && v
}

func TestEndpointIPAMConfigWithOutOfRangeAddrs(t *testing.T) {
	tests := []struct {
		name           string
		ipamConfig     *EndpointIPAMConfig
		v4Subnets      []NetworkSubnet
		v6Subnets      []NetworkSubnet
		expectedErrors []string
	}{
		{
			name: "valid config",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  netip.MustParseAddr("192.168.100.10"),
				IPv6Address:  netip.MustParseAddr("2a01:d2:af:420b:25c1:1816:bb33:855c"),
				LinkLocalIPs: []netip.Addr{netip.MustParseAddr("169.254.169.254"), netip.MustParseAddr("fe80::42:a8ff:fe33:6230")},
			},
			v4Subnets: []NetworkSubnet{
				subnetStub{static: true, contains: map[string]bool{"192.168.100.10": true}},
			},
			v6Subnets: []NetworkSubnet{
				subnetStub{static: true, contains: map[string]bool{"2a01:d2:af:420b:25c1:1816:bb33:855c": true}},
			},
		},
		{
			name: "static addresses out of range",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address: netip.MustParseAddr("192.168.100.10"),
				IPv6Address: netip.MustParseAddr("2a01:d2:af:420b:25c1:1816:bb33:855c"),
			},
			v4Subnets: []NetworkSubnet{
				subnetStub{static: true, contains: map[string]bool{"192.168.100.10": false}},
			},
			v6Subnets: []NetworkSubnet{
				subnetStub{static: true, contains: map[string]bool{"2a01:d2:af:420b:25c1:1816:bb33:855c": false}},
			},
			expectedErrors: []string{
				"no configured subnet or ip-range contain the IP address 192.168.100.10",
				"no configured subnet or ip-range contain the IP address 2a01:d2:af:420b:25c1:1816:bb33:855c",
			},
		},
		{
			name: "static addresses with dynamic network subnets",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address: netip.MustParseAddr("192.168.100.10"),
				IPv6Address: netip.MustParseAddr("2a01:d2:af:420b:25c1:1816:bb33:855c"),
			},
			v4Subnets: []NetworkSubnet{
				subnetStub{static: false},
			},
			v6Subnets: []NetworkSubnet{
				subnetStub{static: false},
			},
			expectedErrors: []string{
				"user specified IP address is supported only when connecting to networks with user configured subnets",
				"user specified IP address is supported only when connecting to networks with user configured subnets",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.ipamConfig.IsInRange(tc.v4Subnets, tc.v6Subnets)
			if tc.expectedErrors == nil {
				assert.NilError(t, err)
				return
			}

			if _, ok := err.(interface{ Unwrap() []error }); !ok {
				t.Fatal("returned error isn't a multierror")
			}
			errs := err.(interface{ Unwrap() []error }).Unwrap()
			assert.Check(t, len(errs) == len(tc.expectedErrors), "errs: %+v", errs)

			for _, expected := range tc.expectedErrors {
				assert.Check(t, is.ErrorContains(err, expected))
			}
		})
	}
}

func TestEndpointIPAMConfigWithInvalidConfig(t *testing.T) {
	tests := []struct {
		name           string
		ipamConfig     *EndpointIPAMConfig
		expectedErrors []string
	}{
		{
			name: "valid config",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  netip.MustParseAddr("192.168.100.10"),
				IPv6Address:  netip.MustParseAddr("2a01:d2:af:420b:25c1:1816:bb33:855c"),
				LinkLocalIPs: []netip.Addr{netip.MustParseAddr("169.254.169.254"), netip.MustParseAddr("fe80::42:a8ff:fe33:6230")},
			},
		},
		{
			name: "invalid IP addresses",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address: netip.MustParseAddr("2001::1"),
				IPv6Address: netip.MustParseAddr("1.2.3.4"),
			},
			expectedErrors: []string{
				"invalid IPv4 address: 2001::1",
				"invalid IPv6 address: 1.2.3.4",
			},
		},
		{
			name:       "ipv6 address with a zone",
			ipamConfig: &EndpointIPAMConfig{IPv6Address: netip.MustParseAddr("fe80::1cc0:3e8c:119f:c2e1%ens18")},
			expectedErrors: []string{
				"invalid IPv6 address: fe80::1cc0:3e8c:119f:c2e1%ens18",
			},
		},
		{
			name: "unspecified address is invalid",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  netip.IPv4Unspecified(),
				IPv6Address:  netip.IPv6Unspecified(),
				LinkLocalIPs: []netip.Addr{netip.IPv4Unspecified(), netip.IPv6Unspecified()},
			},
			expectedErrors: []string{
				"invalid IPv4 address: 0.0.0.0",
				"invalid IPv6 address: ::",
				"invalid link-local IP address: 0.0.0.0",
				"invalid link-local IP address: ::",
			},
		},
		{
			name: "empty link-local",
			ipamConfig: &EndpointIPAMConfig{
				LinkLocalIPs: make([]netip.Addr, 1),
			},
			expectedErrors: []string{"invalid link-local IP address:"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.ipamConfig.Validate()
			if tc.expectedErrors == nil {
				assert.NilError(t, err)
				return
			}

			if _, ok := err.(interface{ Unwrap() []error }); !ok {
				t.Fatalf("returned error isn't a multierror: %v", err)
			}
			errs := err.(interface{ Unwrap() []error }).Unwrap()
			assert.Check(t, len(errs) == len(tc.expectedErrors), "errs: %+v", errs)

			for _, expected := range tc.expectedErrors {
				assert.Check(t, is.ErrorContains(err, expected))
			}
		})
	}
}
