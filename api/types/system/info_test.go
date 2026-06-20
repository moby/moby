package system_test

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/moby/moby/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoUnmarshalInvalidDefaultAddressPoolPrefix(t *testing.T) {
	const body = `{
		"Name": "broken-daemon",
		"ServerVersion": "28.5.0",
		"DefaultAddressPools": [
			{"Base": "invalid Prefix", "Size": 0}
		]
	}`

	var info system.Info
	err := json.Unmarshal([]byte(body), &info)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(info.Name, "broken-daemon"))
	assert.Check(t, is.Equal(info.ServerVersion, "28.5.0"))
	assert.Check(t, is.Len(info.DefaultAddressPools, 1))
	assert.Check(t, !info.DefaultAddressPools[0].Base.IsValid())
	assert.Check(t, is.Equal(info.DefaultAddressPools[0].Size, 0))
}

func TestNetworkAddressPoolUnmarshalInvalidPrefixResetsBase(t *testing.T) {
	pool := system.NetworkAddressPool{
		Base: netip.MustParsePrefix("10.10.0.0/16"),
		Size: 24,
	}

	err := json.Unmarshal([]byte(`{"Base":"invalid Prefix","Size":0}`), &pool)
	assert.NilError(t, err)
	assert.Check(t, !pool.Base.IsValid())
	assert.Check(t, is.Equal(pool.Size, 0))
}

func TestNetworkAddressPoolUnmarshalInvalidPrefixWithNonZeroSize(t *testing.T) {
	var pool system.NetworkAddressPool
	err := json.Unmarshal([]byte(`{"Base":"invalid Prefix","Size":24}`), &pool)
	assert.Check(t, is.ErrorContains(err, "netip.ParsePrefix"))
}

func TestInfoUnmarshalDefaultAddressPoolPrefix(t *testing.T) {
	const body = `{
		"DefaultAddressPools": [
			{"Base": "10.10.0.0/16", "Size": 24}
		]
	}`

	var info system.Info
	err := json.Unmarshal([]byte(body), &info)
	assert.NilError(t, err)
	assert.Check(t, is.Len(info.DefaultAddressPools, 1))
	assert.Check(t, is.Equal(info.DefaultAddressPools[0].Base.String(), "10.10.0.0/16"))
	assert.Check(t, is.Equal(info.DefaultAddressPools[0].Size, 24))
}

func TestInfoUnmarshalMalformedDefaultAddressPoolPrefix(t *testing.T) {
	const body = `{
		"DefaultAddressPools": [
			{"Base": "not-a-prefix", "Size": 24}
		]
	}`

	var info system.Info
	err := json.Unmarshal([]byte(body), &info)
	assert.Check(t, is.ErrorContains(err, "netip.ParsePrefix"))
}
