package network

import (
	"net"
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

func (stub subnetStub) Contains(addr net.IP) bool {
	v, ok := stub.contains[addr.String()]
	return ok && v
}

func TestEndpointIPAMConfigWithOutOfRangeAddrs(t *testing.T) {
	testcases := []struct {
		name           string
		ipamConfig     *EndpointIPAMConfig
		v4Subnets      []NetworkSubnet
		v6Subnets      []NetworkSubnet
		expectedErrors []string
	}{
		{
			name: "valid config",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  "192.168.100.10",
				IPv6Address:  "2a01:d2:af:420b:25c1:1816:bb33:855c",
				LinkLocalIPs: []string{"169.254.169.254", "fe80::42:a8ff:fe33:6230"},
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
				IPv4Address: "192.168.100.10",
				IPv6Address: "2a01:d2:af:420b:25c1:1816:bb33:855c",
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
				IPv4Address: "192.168.100.10",
				IPv6Address: "2a01:d2:af:420b:25c1:1816:bb33:855c",
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

	for _, tc := range testcases {
		tc := tc
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
	testcases := []struct {
		name           string
		ipamConfig     *EndpointIPAMConfig
		expectedErrors []string
	}{
		{
			name: "valid config",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  "192.168.100.10",
				IPv6Address:  "2a01:d2:af:420b:25c1:1816:bb33:855c",
				LinkLocalIPs: []string{"169.254.169.254", "fe80::42:a8ff:fe33:6230"},
			},
		},
		{
			name: "invalid IP addresses",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  "foo",
				IPv6Address:  "bar",
				LinkLocalIPs: []string{"baz", "foobar"},
			},
			expectedErrors: []string{
				"invalid IPv4 address: foo",
				"invalid IPv6 address: bar",
				"invalid link-local IP address: baz",
				"invalid link-local IP address: foobar",
			},
		},
		{
			name:       "ipv6 address with a zone",
			ipamConfig: &EndpointIPAMConfig{IPv6Address: "fe80::1cc0:3e8c:119f:c2e1%ens18"},
			expectedErrors: []string{
				"invalid IPv6 address: fe80::1cc0:3e8c:119f:c2e1%ens18",
			},
		},
		{
			name: "unspecified address is invalid",
			ipamConfig: &EndpointIPAMConfig{
				IPv4Address:  "0.0.0.0",
				IPv6Address:  "::",
				LinkLocalIPs: []string{"0.0.0.0", "::"},
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
				LinkLocalIPs: []string{""},
			},
			expectedErrors: []string{"invalid link-local IP address:"},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.ipamConfig.Validate()
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
