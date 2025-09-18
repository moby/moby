package daemon

import (
	"encoding/json"
	"errors"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDNSNamesOrder(t *testing.T) {
	d := &Daemon{}
	ctr := &container.Container{
		ID:   "35de8003b19e27f636fc6ecbf4d7072558b872a8544f287fd69ad8182ad59023",
		Name: "foobar",
		Config: &containertypes.Config{
			Hostname: "baz",
		},
		HostConfig: &containertypes.HostConfig{},
	}
	nw := buildNetwork(t, map[string]any{
		"id":          "1234567890",
		"name":        "testnet",
		"networkType": "bridge",
		"enableIPv6":  false,
	})
	epSettings := &networktypes.EndpointSettings{
		Aliases: []string{"myctr"},
	}

	if err := d.updateNetworkConfig(ctr, nw, epSettings); err != nil {
		t.Fatal(err)
	}

	assert.Check(t, is.DeepEqual(epSettings.DNSNames, []string{"foobar", "myctr", "35de8003b19e", "baz"}))
}

func buildNetwork(t *testing.T, config map[string]any) *libnetwork.Network {
	t.Helper()

	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	nw := &libnetwork.Network{}
	if err := nw.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}

	return nw
}

func TestEndpointIPAMConfigWithOutOfRangeAddrs(t *testing.T) {
	tests := []struct {
		name           string
		ipamConfig     *networktypes.EndpointIPAMConfig
		v4Subnets      []*libnetwork.IpamConf
		v6Subnets      []*libnetwork.IpamConf
		expectedErrors []string
	}{
		{
			name: "valid config",
			ipamConfig: &networktypes.EndpointIPAMConfig{
				IPv4Address:  "192.168.100.10",
				IPv6Address:  "2a01:d2:af:420b:25c1:1816:bb33:855c",
				LinkLocalIPs: []string{"169.254.169.254", "fe80::42:a8ff:fe33:6230"},
			},
			v4Subnets: []*libnetwork.IpamConf{
				{PreferredPool: "192.168.100.0/24"},
			},
			v6Subnets: []*libnetwork.IpamConf{
				{PreferredPool: "2a01:d2:af:420b:25c1:1816:bb33::/112"},
			},
		},
		{
			name: "static addresses out of range",
			ipamConfig: &networktypes.EndpointIPAMConfig{
				IPv4Address: "192.168.100.10",
				IPv6Address: "2a01:d2:af:420b:25c1:1816:bb33:855c",
			},
			v4Subnets: []*libnetwork.IpamConf{
				{PreferredPool: "192.168.255.0/24"},
			},
			v6Subnets: []*libnetwork.IpamConf{
				{PreferredPool: "2001:db8::/112"},
			},
			expectedErrors: []string{
				"no configured subnet or ip-range contain the IP address 192.168.100.10",
				"no configured subnet or ip-range contain the IP address 2a01:d2:af:420b:25c1:1816:bb33:855c",
			},
		},
		{
			name: "static addresses with dynamic network subnets",
			ipamConfig: &networktypes.EndpointIPAMConfig{
				IPv4Address: "192.168.100.10",
				IPv6Address: "2a01:d2:af:420b:25c1:1816:bb33:855c",
			},
			v4Subnets: []*libnetwork.IpamConf{
				{},
			},
			v6Subnets: []*libnetwork.IpamConf{
				{},
			},
			expectedErrors: []string{
				"user specified IP address is supported only when connecting to networks with user configured subnets",
				"user specified IP address is supported only when connecting to networks with user configured subnets",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateIPAMConfigIsInRange(nil, tc.ipamConfig, tc.v4Subnets, tc.v6Subnets)
			if tc.expectedErrors == nil {
				assert.NilError(t, errors.Join(errs...))
				return
			}

			assert.Check(t, len(errs) == len(tc.expectedErrors), "errs: %+v", errs)

			err := errors.Join(errs...)
			for _, expected := range tc.expectedErrors {
				assert.Check(t, is.ErrorContains(err, expected))
			}
		})
	}
}

func TestEndpointIPAMConfigWithInvalidConfig(t *testing.T) {
	tests := []struct {
		name           string
		ipamConfig     *networktypes.EndpointIPAMConfig
		expectedErrors []string
	}{
		{
			name: "valid config",
			ipamConfig: &networktypes.EndpointIPAMConfig{
				IPv4Address:  "192.168.100.10",
				IPv6Address:  "2a01:d2:af:420b:25c1:1816:bb33:855c",
				LinkLocalIPs: []string{"169.254.169.254", "fe80::42:a8ff:fe33:6230"},
			},
		},
		{
			name: "invalid IP addresses",
			ipamConfig: &networktypes.EndpointIPAMConfig{
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
			ipamConfig: &networktypes.EndpointIPAMConfig{IPv6Address: "fe80::1cc0:3e8c:119f:c2e1%ens18"},
			expectedErrors: []string{
				"invalid IPv6 address: fe80::1cc0:3e8c:119f:c2e1%ens18",
			},
		},
		{
			name: "unspecified address is invalid",
			ipamConfig: &networktypes.EndpointIPAMConfig{
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
			ipamConfig: &networktypes.EndpointIPAMConfig{
				LinkLocalIPs: []string{""},
			},
			expectedErrors: []string{"invalid link-local IP address:"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateEndpointIPAMConfig(nil, tc.ipamConfig)
			if tc.expectedErrors == nil {
				assert.NilError(t, errors.Join(errs...))
				return
			}

			assert.Check(t, len(errs) == len(tc.expectedErrors), "errs: %+v", errs)

			err := errors.Join(errs...)
			for _, expected := range tc.expectedErrors {
				assert.Check(t, is.ErrorContains(err, expected))
			}
		})
	}
}
