package opts

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestHostGateway(t *testing.T) {
	testcases := []struct {
		name   string
		input  string
		expStr string
		expErr string
	}{
		{
			name:   "ipv4",
			input:  "10.1.1.1",
			expStr: "10.1.1.1",
		},
		{
			name:   "ipv6",
			input:  "fdb8:3037:2cb2::1",
			expStr: "fdb8:3037:2cb2::1",
		},
		{
			name:   "ipv6 and ipv4",
			input:  "fdb8:3037:2cb2::1,10.1.1.1",
			expStr: "10.1.1.1,fdb8:3037:2cb2::1",
		},
		{
			name:   "not an IP address",
			input:  "blah",
			expErr: `invalid IP address "blah" in option host-gateway-ip`,
		},
		{
			name:   "missing address",
			input:  "10.1.1.1,",
			expErr: `invalid IP address "" in option host-gateway-ip`,
		},
		{
			name:   "no address",
			input:  "",
			expErr: `invalid IP address "" in option host-gateway-ip`,
		},
		{
			name:   "two ipv4",
			input:  "10.1.1.1,10.1.1.2",
			expErr: "at most one IPv4 address is allowed in option host-gateway-ip",
		},
		{
			name:   "two ipv6",
			input:  "fdb8:3037:2cb2::1,fdb8:3037:2cb2::2",
			expErr: "at most one IPv6 address is allowed in option host-gateway-ip",
		},
		{
			name:   "space in input",
			input:  "fdb8:3037:2cb2::1, 10.1.1.1",
			expErr: `invalid IP address " 10.1.1.1" in option host-gateway-ip`,
		},
	}

	for _, tc := range testcases {
		testSetHostGateway(t, tc.name, tc.input, tc.expStr, tc.expErr)
		testUnmarshalHostGateway(t, tc.name, []byte(`"`+tc.input+`"`), tc.expStr, tc.expErr)
	}
}

func TestHostGatewayBadJSON(t *testing.T) {
	testUnmarshalHostGateway(t, "not a json string", []byte(`["10.1.1.1"]`),
		"", "invalid host-gateway-ip option")
}

// TestHostGatewayMultiOpts checks that it's possible to accumulate addresses
// from multiple command line options. For example:
//
//	--host-gateway-ip 10.1.1.1 --host-gateway-ip fdb8:3037:2cb2::1
func TestHostGatewayMultiOpts(t *testing.T) {
	var hg HostGateway
	err := hg.Set("10.1.1.1")
	assert.Check(t, err)
	err = hg.Set("fdb8:3037:2cb2::1")
	assert.Check(t, err)
	assert.Check(t, is.Equal(hg.String(), "10.1.1.1,fdb8:3037:2cb2::1"))
	err = hg.Set("fdb8:3037:2cb2::2")
	assert.Check(t, is.Error(err, "at most one IPv6 address is allowed in option host-gateway-ip"))
}

func testSetHostGateway(t *testing.T, name, setVal, expStr, expErr string) {
	t.Helper()
	var hg HostGateway
	err := hg.Set(setVal)
	if expErr != "" {
		assert.Check(t, is.Error(err, expErr), "set %s", name)
		return
	}
	assert.Check(t, is.Equal(hg.String(), expStr), "set %s", name)
}

func testUnmarshalHostGateway(t *testing.T, name string, jsonVal []byte, expStr, expErr string) {
	t.Helper()
	var hg HostGateway
	err := hg.UnmarshalJSON(jsonVal)
	if expErr != "" {
		assert.Check(t, is.ErrorContains(err, expErr), "unmarshal %s", name)
		return
	}
	assert.Check(t, is.Equal(hg.String(), expStr), "unmarshal %s", name)
}
