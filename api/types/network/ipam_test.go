package network_test

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestIPAMConfig_UnmarshalJSON verifies that IPAMConfig.UnmarshalJSON parses
// normal addresses and, for backwards compatibility with Docker <=26.x daemons
// that stored gateway addresses with CIDR prefix notation, also accepts
// addresses such as "fd05:d0ca:2::1/112" by silently stripping the prefix.
func TestIPAMConfig_UnmarshalJSON(t *testing.T) {
	t.Run("normal IPv4 gateway", func(t *testing.T) {
		data := `{"Subnet":"172.19.0.0/24","Gateway":"172.19.0.1"}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.Gateway, netip.MustParseAddr("172.19.0.1")))
		assert.Check(t, is.Equal(c.Subnet, netip.MustParsePrefix("172.19.0.0/24")))
	})

	t.Run("normal IPv6 gateway", func(t *testing.T) {
		data := `{"Subnet":"fd05:d0ca:2::/112","Gateway":"fd05:d0ca:2::1"}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.Gateway, netip.MustParseAddr("fd05:d0ca:2::1")))
	})

	// Older Docker daemons (<=26.x) stored gateway addresses with a CIDR prefix.
	// The client should silently strip the prefix and return just the host address.
	// See: https://github.com/moby/moby/issues/52991
	t.Run("legacy IPv6 gateway with prefix (backwards compat)", func(t *testing.T) {
		data := `{"Subnet":"fd05:d0ca:2::/112","Gateway":"fd05:d0ca:2::1/112"}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.Gateway, netip.MustParseAddr("fd05:d0ca:2::1")))
	})

	t.Run("legacy IPv4 gateway with prefix (backwards compat)", func(t *testing.T) {
		data := `{"Subnet":"172.19.0.0/24","Gateway":"172.19.0.1/24"}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.Gateway, netip.MustParseAddr("172.19.0.1")))
	})

	t.Run("empty gateway", func(t *testing.T) {
		data := `{"Subnet":"172.19.0.0/24","Gateway":""}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, !c.Gateway.IsValid())
	})

	t.Run("omitted gateway", func(t *testing.T) {
		data := `{"Subnet":"172.19.0.0/24"}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, !c.Gateway.IsValid())
	})

	t.Run("auxiliary addresses with legacy CIDR prefix (backwards compat)", func(t *testing.T) {
		data := `{"AuxiliaryAddresses":{"router":"192.168.1.254/24","secondary":"192.168.1.253/24"}}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.AuxAddress["router"], netip.MustParseAddr("192.168.1.254")))
		assert.Check(t, is.Equal(c.AuxAddress["secondary"], netip.MustParseAddr("192.168.1.253")))
	})

	t.Run("auxiliary addresses without prefix", func(t *testing.T) {
		data := `{"AuxiliaryAddresses":{"router":"192.168.1.254"}}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.AuxAddress["router"], netip.MustParseAddr("192.168.1.254")))
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		data := `{"Gateway": 12345}`
		var c network.IPAMConfig
		assert.Check(t, json.Unmarshal([]byte(data), &c) != nil)
	})

	// IPv4-mapped IPv6 addresses should be unwrapped to their plain IPv4 form.
	t.Run("IPv4-mapped IPv6 gateway is unmapped", func(t *testing.T) {
		data := `{"Gateway":"::ffff:172.19.0.1"}`
		var c network.IPAMConfig
		assert.NilError(t, json.Unmarshal([]byte(data), &c))
		assert.Check(t, is.Equal(c.Gateway, netip.MustParseAddr("172.19.0.1")))
	})

	// Full round-trip: marshal then unmarshal should preserve Gateway.
	t.Run("round-trip marshal unmarshal", func(t *testing.T) {
		orig := network.IPAMConfig{
			Subnet:  netip.MustParsePrefix("172.19.0.0/24"),
			Gateway: netip.MustParseAddr("172.19.0.1"),
		}
		data, err := json.Marshal(orig)
		assert.NilError(t, err)

		var got network.IPAMConfig
		assert.NilError(t, json.Unmarshal(data, &got))
		assert.Check(t, is.Equal(got.Subnet, orig.Subnet))
		assert.Check(t, is.Equal(got.Gateway, orig.Gateway))
	})
}
