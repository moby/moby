package registry

import (
	"errors"
	"net"
	"testing"

	"github.com/distribution/reference"
	"gotest.tools/v3/assert"
)

// overrideLookupIP overrides net.LookupIP for testing.
func overrideLookupIP(t *testing.T) {
	t.Helper()
	restoreLookup := lookupIP

	// override net.LookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		mockHosts := map[string][]net.IP{
			"":            {net.ParseIP("0.0.0.0")},
			"localhost":   {net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
			"example.com": {net.ParseIP("192.0.2.42")},
			"other.com":   {net.ParseIP("198.51.100.43")},
			"loopback.example": {
				net.ParseIP("127.0.0.2"),
				net.ParseIP("::1"),
			},
			"mixed.example": {
				net.ParseIP("127.0.0.1"),
				net.ParseIP("192.0.2.1"),
			},
		}
		if addrs, ok := mockHosts[host]; ok {
			return addrs, nil
		}
		return nil, errors.New("lookup: no such host")
	}
	t.Cleanup(func() {
		lookupIP = restoreLookup
	})
}

func TestMirrorEndpointLookup(t *testing.T) {
	containsMirror := func(endpoints []APIEndpoint) bool {
		for _, pe := range endpoints {
			if pe.URL.Host == "my.mirror" {
				return true
			}
		}
		return false
	}
	cfg, err := newServiceConfig(ServiceOptions{
		Mirrors: []string{"https://my.mirror"},
	})
	assert.NilError(t, err)
	s := Service{config: cfg}

	imageName, err := reference.WithName(IndexName + "/test/image")
	if err != nil {
		t.Error(err)
	}
	pushAPIEndpoints, err := s.LookupPushEndpoints(reference.Domain(imageName))
	if err != nil {
		t.Fatal(err)
	}
	if containsMirror(pushAPIEndpoints) {
		t.Fatal("Push endpoint should not contain mirror")
	}

	pullAPIEndpoints, err := s.LookupPullEndpoints(reference.Domain(imageName))
	if err != nil {
		t.Fatal(err)
	}
	if !containsMirror(pullAPIEndpoints) {
		t.Fatal("Pull endpoint should contain mirror")
	}
}

func TestIsSecureIndex(t *testing.T) {
	overrideLookupIP(t)
	tests := []struct {
		name     string
		addr     string
		insecure []string
		expected bool
	}{
		{
			name:     "official registry",
			addr:     IndexName,
			expected: true,
		},
		{
			name:     "registry without insecure registries",
			addr:     "example.com",
			expected: true,
		},
		{
			name:     "registry configured as insecure",
			addr:     "example.com",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "localhost with different configured port",
			addr:     "localhost",
			insecure: []string{"localhost:5000"},
			expected: false,
		},
		{
			name:     "localhost with configured port",
			addr:     "localhost:5000",
			insecure: []string{"localhost:5000"},
			expected: false,
		},
		{
			name:     "localhost with unrelated insecure registry",
			addr:     "localhost",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with configured port",
			addr:     "127.0.0.1:5000",
			insecure: []string{"127.0.0.1:5000"},
			expected: false,
		},
		{
			name:     "localhost is insecure by default",
			addr:     "localhost",
			expected: false,
		},
		{
			name:     "localhost with port is insecure by default",
			addr:     "localhost:5000",
			expected: false,
		},
		{
			name:     "loopback address is insecure by default",
			addr:     "127.0.0.1",
			expected: false,
		},
		{
			name:     "localhost with unrelated insecure registry duplicate",
			addr:     "localhost",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with unrelated insecure registry",
			addr:     "127.0.0.1",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "registry is secure by default",
			addr:     "example.com",
			expected: true,
		},
		{
			name:     "registry configured as insecure duplicate",
			addr:     "example.com",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with unrelated insecure registry duplicate",
			addr:     "127.0.0.1",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with port and unrelated insecure registry",
			addr:     "127.0.0.1:5000",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "registry with port matching insecure CIDR",
			addr:     "example.com:5000",
			insecure: []string{"192.0.2.0/24"},
			expected: false,
		},
		{
			name:     "registry matching insecure CIDR",
			addr:     "example.com",
			insecure: []string{"192.0.2.0/24"},
			expected: false,
		},
		{
			name:     "registry matching masked insecure CIDR",
			addr:     "example.com:5000",
			insecure: []string{"192.0.2.42/24"},
			expected: false,
		},
		{
			name:     "loopback address matching insecure CIDR",
			addr:     "127.0.0.1:5000",
			insecure: []string{"127.0.0.0/8"},
			expected: false,
		},
		{
			name:     "IP address matching masked insecure CIDR",
			addr:     "192.0.2.42:5000",
			insecure: []string{"192.0.2.1/24"},
			expected: false,
		},
		{
			name:     "unresolvable registry does not match insecure CIDR",
			addr:     "invalid.example.com",
			insecure: []string{"192.0.2.0/24"},
			expected: true,
		},
		{
			name:     "unresolvable registry configured as insecure",
			addr:     "invalid.example.com",
			insecure: []string{"invalid.example.com"},
			expected: false,
		},
		{
			name:     "unresolvable registry with port does not match hostname",
			addr:     "invalid.example.com:5000",
			insecure: []string{"invalid.example.com"},
			expected: true,
		},
		{
			name:     "unresolvable registry with port configured as insecure",
			addr:     "invalid.example.com:5000",
			insecure: []string{"invalid.example.com:5000"},
			expected: false,
		},
		{
			name:     "all resolved addresses in insecure subnets are insecure",
			addr:     "loopback.example",
			insecure: []string{"127.0.0.0/8", "::1/128"},
			expected: false,
		},
		{
			name:     "resolved addresses may match different insecure subnets",
			addr:     "mixed.example",
			insecure: []string{"127.0.0.0/8", "192.0.2.0/24"},
			expected: false,
		},
		{
			name:     "registry is secure if any resolved address is outside insecure subnets",
			addr:     "mixed.example",
			insecure: []string{"127.0.0.0/8"},
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := newServiceConfig(ServiceOptions{
				InsecureRegistries: tc.insecure,
			})
			assert.NilError(t, err)

			sec := config.isSecureIndex(tc.addr)
			assert.Equal(t, sec, tc.expected)
		})
	}
}
