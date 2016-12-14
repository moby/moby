package swarm

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestNodeAddrOptionSetHostAndPort(t *testing.T) {
	opt := NewNodeAddrOption("old:123")
	addr := "newhost:5555"
	assert.NilError(t, opt.Set(addr))
	assert.Equal(t, opt.Value(), addr)
}

func TestNodeAddrOptionSetHostOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("newhost"))
	assert.Equal(t, opt.Value(), "newhost:2377")
}

func TestNodeAddrOptionSetHostOnlyIPv6(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("::1"))
	assert.Equal(t, opt.Value(), "[::1]:2377")
}

func TestNodeAddrOptionSetPortOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set(":4545"))
	assert.Equal(t, opt.Value(), "0.0.0.0:4545")
}

func TestNodeAddrOptionSetInvalidFormat(t *testing.T) {
	opt := NewListenAddrOption()
	assert.Error(t, opt.Set("http://localhost:4545"), "Invalid")
}

func TestExternalCAOptionErrors(t *testing.T) {
	testCases := []struct {
		externalCA    string
		expectedError string
	}{
		{
			externalCA:    "",
			expectedError: "EOF",
		},
		{
			externalCA:    "anything",
			expectedError: "invalid field 'anything' must be a key=value pair",
		},
		{
			externalCA:    "foo=bar",
			expectedError: "the external-ca option needs a protocol= parameter",
		},
		{
			externalCA:    "protocol=baz",
			expectedError: "unrecognized external CA protocol baz",
		},
		{
			externalCA:    "protocol=cfssl",
			expectedError: "the external-ca option needs a url= parameter",
		},
	}
	for _, tc := range testCases {
		opt := &ExternalCAOption{}
		assert.Error(t, opt.Set(tc.externalCA), tc.expectedError)
	}
}

func TestExternalCAOption(t *testing.T) {
	testCases := []struct {
		externalCA string
		expected   string
	}{
		{
			externalCA: "protocol=cfssl,url=anything",
			expected:   "cfssl: anything",
		},
		{
			externalCA: "protocol=CFSSL,url=anything",
			expected:   "cfssl: anything",
		},
		{
			externalCA: "protocol=Cfssl,url=https://example.com",
			expected:   "cfssl: https://example.com",
		},
		{
			externalCA: "protocol=Cfssl,url=https://example.com,foo=bar",
			expected:   "cfssl: https://example.com",
		},
		{
			externalCA: "protocol=Cfssl,url=https://example.com,foo=bar,foo=baz",
			expected:   "cfssl: https://example.com",
		},
	}
	for _, tc := range testCases {
		opt := &ExternalCAOption{}
		assert.NilError(t, opt.Set(tc.externalCA))
		assert.Equal(t, opt.String(), tc.expected)
	}
}

func TestExternalCAOptionMultiple(t *testing.T) {
	opt := &ExternalCAOption{}
	assert.NilError(t, opt.Set("protocol=cfssl,url=https://example.com"))
	assert.NilError(t, opt.Set("protocol=CFSSL,url=anything"))
	assert.Equal(t, len(opt.Value()), 2)
	assert.Equal(t, opt.String(), "cfssl: https://example.com, cfssl: anything")
}
