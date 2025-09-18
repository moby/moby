package daemon

import (
	"testing"

	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateIPAM(t *testing.T) {
	tests := []struct {
		name           string
		ipam           []network.IPAMConfig
		ipv6           bool
		expectedErrors []string
	}{
		{
			name: "IP version mismatch",
			ipam: []network.IPAMConfig{{
				Subnet:     "10.10.10.0/24",
				IPRange:    "2001:db8::/32",
				Gateway:    "2001:db8::1",
				AuxAddress: map[string]string{"DefaultGatewayIPv4": "2001:db8::1"},
			}},
			expectedErrors: []string{
				"invalid ip-range 2001:db8::/32: parent subnet is an IPv4 block",
				"invalid gateway 2001:db8::1: parent subnet is an IPv4 block",
				"invalid auxiliary address DefaultGatewayIPv4: parent subnet is an IPv4 block",
			},
		},
		{
			// Regression test for https://github.com/moby/moby/issues/47202
			name: "IPv6 subnet is discarded with no error when IPv6 is disabled",
			ipam: []network.IPAMConfig{{Subnet: "2001:db8::/32"}},
			ipv6: false,
		},
		{
			name: "Invalid data - Subnet",
			ipam: []network.IPAMConfig{{Subnet: "foobar"}},
			expectedErrors: []string{
				`invalid subnet foobar: invalid CIDR block notation`,
			},
		},
		{
			name: "Invalid data",
			ipam: []network.IPAMConfig{{
				Subnet:     "10.10.10.0/24",
				IPRange:    "foobar",
				Gateway:    "1001.10.5.3",
				AuxAddress: map[string]string{"DefaultGatewayIPv4": "dummy"},
			}},
			expectedErrors: []string{
				"invalid ip-range foobar: invalid CIDR block notation",
				"invalid gateway 1001.10.5.3: invalid address",
				"invalid auxiliary address DefaultGatewayIPv4: invalid address",
			},
		},
		{
			name: "IPRange bigger than its subnet",
			ipam: []network.IPAMConfig{
				{Subnet: "10.10.10.0/24", IPRange: "10.0.0.0/8"},
			},
			expectedErrors: []string{
				"invalid ip-range 10.0.0.0/8: CIDR block is bigger than its parent subnet 10.10.10.0/24",
			},
		},
		{
			name: "Out of range prefix & addresses",
			ipam: []network.IPAMConfig{{
				Subnet:     "10.0.0.0/8",
				IPRange:    "192.168.0.1/24",
				Gateway:    "192.168.0.1",
				AuxAddress: map[string]string{"DefaultGatewayIPv4": "192.168.0.1"},
			}},
			expectedErrors: []string{
				"invalid ip-range 192.168.0.1/24: it should be 192.168.0.0/24",
				"invalid ip-range 192.168.0.1/24: parent subnet 10.0.0.0/8 doesn't contain ip-range",
				"invalid gateway 192.168.0.1: parent subnet 10.0.0.0/8 doesn't contain this address",
				"invalid auxiliary address DefaultGatewayIPv4: parent subnet 10.0.0.0/8 doesn't contain this address",
			},
		},
		{
			name: "Subnet with host fragment set",
			ipam: []network.IPAMConfig{{
				Subnet: "10.10.10.0/8",
			}},
			expectedErrors: []string{"invalid subnet 10.10.10.0/8: it should be 10.0.0.0/8"},
		},
		{
			name: "IPRange with host fragment set",
			ipam: []network.IPAMConfig{{
				Subnet:  "10.0.0.0/8",
				IPRange: "10.10.10.0/16",
			}},
			expectedErrors: []string{"invalid ip-range 10.10.10.0/16: it should be 10.10.0.0/16"},
		},
		{
			name: "Empty IPAM is valid",
		},
		{
			name: "Valid IPAM",
			ipam: []network.IPAMConfig{{
				Subnet:     "10.0.0.0/8",
				IPRange:    "10.10.0.0/16",
				Gateway:    "10.10.0.1",
				AuxAddress: map[string]string{"DefaultGatewayIPv4": "10.10.0.1"},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errs := validateIpamConfig(tc.ipam, tc.ipv6)
			if tc.expectedErrors == nil {
				assert.NilError(t, errs)
				return
			}

			assert.Check(t, is.ErrorContains(errs, "invalid network config"))
			for _, expected := range tc.expectedErrors {
				assert.Check(t, is.ErrorContains(errs, expected))
			}
		})
	}
}
