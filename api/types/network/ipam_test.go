package network

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworkWithInvalidIPAM(t *testing.T) {
	testcases := []struct {
		name           string
		ipam           IPAM
		ipv6           bool
		expectedErrors []string
	}{
		{
			name: "IP version mismatch",
			ipam: IPAM{
				Config: []IPAMConfig{{
					Subnet:     "10.10.10.0/24",
					IPRange:    "2001:db8::/32",
					Gateway:    "2001:db8::1",
					AuxAddress: map[string]string{"DefaultGatewayIPv4": "2001:db8::1"},
				}},
			},
			expectedErrors: []string{
				"invalid ip-range 2001:db8::/32: parent subnet is an IPv4 block",
				"invalid gateway 2001:db8::1: parent subnet is an IPv4 block",
				"invalid auxiliary address DefaultGatewayIPv4: parent subnet is an IPv4 block",
			},
		},
		{
			name:           "IPv6 subnet is discarded when IPv6 is disabled",
			ipam:           IPAM{Config: []IPAMConfig{{Subnet: "2001:db8::/32"}}},
			ipv6:           false,
			expectedErrors: []string{"invalid subnet 2001:db8::/32: IPv6 has not been enabled for this network"},
		},
		{
			name: "Invalid data - Subnet",
			ipam: IPAM{Config: []IPAMConfig{{Subnet: "foobar"}}},
			expectedErrors: []string{
				`invalid subnet foobar: invalid CIDR block notation`,
			},
		},
		{
			name: "Invalid data",
			ipam: IPAM{
				Config: []IPAMConfig{{
					Subnet:     "10.10.10.0/24",
					IPRange:    "foobar",
					Gateway:    "1001.10.5.3",
					AuxAddress: map[string]string{"DefaultGatewayIPv4": "dummy"},
				}},
			},
			expectedErrors: []string{
				"invalid ip-range foobar: invalid CIDR block notation",
				"invalid gateway 1001.10.5.3: invalid address",
				"invalid auxiliary address DefaultGatewayIPv4: invalid address",
			},
		},
		{
			name: "IPRange bigger than its subnet",
			ipam: IPAM{
				Config: []IPAMConfig{
					{Subnet: "10.10.10.0/24", IPRange: "10.0.0.0/8"},
				},
			},
			expectedErrors: []string{
				"invalid ip-range 10.0.0.0/8: CIDR block is bigger than its parent subnet 10.10.10.0/24",
			},
		},
		{
			name: "Out of range prefix & addresses",
			ipam: IPAM{
				Config: []IPAMConfig{{
					Subnet:     "10.0.0.0/8",
					IPRange:    "192.168.0.1/24",
					Gateway:    "192.168.0.1",
					AuxAddress: map[string]string{"DefaultGatewayIPv4": "192.168.0.1"},
				}},
			},
			expectedErrors: []string{
				"invalid ip-range 192.168.0.1/24: it should be 192.168.0.0/24",
				"invalid ip-range 192.168.0.1/24: parent subnet 10.0.0.0/8 doesn't contain ip-range",
				"invalid gateway 192.168.0.1: parent subnet 10.0.0.0/8 doesn't contain this address",
				"invalid auxiliary address DefaultGatewayIPv4: parent subnet 10.0.0.0/8 doesn't contain this address",
			},
		},
		{
			name: "Subnet with host fragment set",
			ipam: IPAM{
				Config: []IPAMConfig{{
					Subnet: "10.10.10.0/8",
				}},
			},
			expectedErrors: []string{"invalid subnet 10.10.10.0/8: it should be 10.0.0.0/8"},
		},
		{
			name: "IPRange with host fragment set",
			ipam: IPAM{
				Config: []IPAMConfig{{
					Subnet:  "10.0.0.0/8",
					IPRange: "10.10.10.0/16",
				}},
			},
			expectedErrors: []string{"invalid ip-range 10.10.10.0/16: it should be 10.10.0.0/16"},
		},
		{
			name: "Empty IPAM is valid",
			ipam: IPAM{},
		},
		{
			name: "Valid IPAM",
			ipam: IPAM{
				Config: []IPAMConfig{{
					Subnet:     "10.0.0.0/8",
					IPRange:    "10.10.0.0/16",
					Gateway:    "10.10.0.1",
					AuxAddress: map[string]string{"DefaultGatewayIPv4": "10.10.0.1"},
				}},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errs := ValidateIPAM(&tc.ipam, tc.ipv6)
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
