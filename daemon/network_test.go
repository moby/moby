package daemon

import (
	"net/netip"
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
				Subnet:     netip.MustParsePrefix("10.10.10.0/24"),
				IPRange:    netip.MustParsePrefix("2001:db8::/32"),
				Gateway:    netip.MustParseAddr("2001:db8::1"),
				AuxAddress: map[string]netip.Addr{"DefaultGatewayIPv4": netip.MustParseAddr("2001:db8::1")},
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
			ipam: []network.IPAMConfig{{Subnet: netip.MustParsePrefix("2001:db8::/32")}},
			ipv6: false,
		},
		{
			name: "IPRange bigger than its subnet",
			ipam: []network.IPAMConfig{
				{Subnet: netip.MustParsePrefix("10.10.10.0/24"), IPRange: netip.MustParsePrefix("10.0.0.0/8")},
			},
			expectedErrors: []string{
				"invalid ip-range 10.0.0.0/8: CIDR block is bigger than its parent subnet 10.10.10.0/24",
			},
		},
		{
			name: "Out of range prefix & addresses",
			ipam: []network.IPAMConfig{{
				Subnet:     netip.MustParsePrefix("10.0.0.0/8"),
				IPRange:    netip.MustParsePrefix("192.168.0.1/24"),
				Gateway:    netip.MustParseAddr("192.168.0.1"),
				AuxAddress: map[string]netip.Addr{"DefaultGatewayIPv4": netip.MustParseAddr("192.168.0.1")},
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
				Subnet: netip.MustParsePrefix("10.10.10.0/8"),
			}},
			expectedErrors: []string{"invalid subnet 10.10.10.0/8: it should be 10.0.0.0/8"},
		},
		{
			name: "IPRange with host fragment set",
			ipam: []network.IPAMConfig{{
				Subnet:  netip.MustParsePrefix("10.0.0.0/8"),
				IPRange: netip.MustParsePrefix("10.10.10.0/16"),
			}},
			expectedErrors: []string{"invalid ip-range 10.10.10.0/16: it should be 10.10.0.0/16"},
		},
		{
			name: "Empty IPAM is valid",
		},
		{
			name: "Valid IPAM",
			ipam: []network.IPAMConfig{{
				Subnet:     netip.MustParsePrefix("10.0.0.0/8"),
				IPRange:    netip.MustParsePrefix("10.10.0.0/16"),
				Gateway:    netip.MustParseAddr("10.10.0.1"),
				AuxAddress: map[string]netip.Addr{"DefaultGatewayIPv4": netip.MustParseAddr("10.10.0.1")},
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
