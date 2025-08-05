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
			"example.com": {net.ParseIP("42.42.42.42")},
			"other.com":   {net.ParseIP("43.43.43.43")},
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
		addr               string
		insecureRegistries []string
		expected           bool
	}{
		{IndexName, nil, true},
		{"example.com", []string{}, true},
		{"example.com", []string{"example.com"}, false},
		{"localhost", []string{"localhost:5000"}, false},
		{"localhost:5000", []string{"localhost:5000"}, false},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"127.0.0.1:5000"}, false},
		{"localhost", nil, false},
		{"localhost:5000", nil, false},
		{"127.0.0.1", nil, false},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1", []string{"example.com"}, false},
		{"example.com", nil, true},
		{"example.com", []string{"example.com"}, false},
		{"127.0.0.1", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"example.com"}, false},
		{"example.com:5000", []string{"42.42.0.0/16"}, false},
		{"example.com", []string{"42.42.0.0/16"}, false},
		{"example.com:5000", []string{"42.42.42.42/8"}, false},
		{"127.0.0.1:5000", []string{"127.0.0.0/8"}, false},
		{"42.42.42.42:5000", []string{"42.1.1.1/8"}, false},
		{"invalid.example.com", []string{"42.42.0.0/16"}, true},
		{"invalid.example.com", []string{"invalid.example.com"}, false},
		{"invalid.example.com:5000", []string{"invalid.example.com"}, true},
		{"invalid.example.com:5000", []string{"invalid.example.com:5000"}, false},
	}
	for _, tc := range tests {
		config, err := newServiceConfig(ServiceOptions{
			InsecureRegistries: tc.insecureRegistries,
		})
		assert.NilError(t, err)

		sec := config.isSecureIndex(tc.addr)
		assert.Equal(t, sec, tc.expected, "isSecureIndex failed for %q %v, expected %v got %v", tc.addr, tc.insecureRegistries, tc.expected, sec)
	}
}
