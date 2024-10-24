package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"github.com/containerd/containerd/remotes/docker"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMirrorsToHosts(t *testing.T) {
	pullCaps := docker.HostCapabilityPull | docker.HostCapabilityResolve
	allCaps := docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush
	defaultRegistry := testRegistryHost("https", "registry-1.docker.com", "/v2", allCaps)
	for _, tc := range []struct {
		mirrors  []string
		dhost    docker.RegistryHost
		expected []docker.RegistryHost
	}{
		{
			mirrors: []string{"https://localhost:5000"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"http://localhost:5000"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("http", "localhost:5000", "/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"http://localhost:5000/v2"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("http", "localhost:5000", "/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"localhost:5000"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"localhost:5000/trailingslash/"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/trailingslash/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"localhost:5000/2trailingslash//"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/2trailingslash/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"localhost:5000/v2/"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			mirrors: []string{"localhost:5000/base"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/base/v2", pullCaps),
				defaultRegistry,
			},
		},
		{
			// Legacy mirror configuration always appended /v2, keep functionality the same
			mirrors: []string{"localhost:5000/v2/base"},
			dhost:   defaultRegistry,
			expected: []docker.RegistryHost{
				testRegistryHost("https", "localhost:5000", "/v2/base/v2", pullCaps),
				defaultRegistry,
			},
		},
	} {
		actual := mirrorsToRegistryHosts(tc.mirrors, tc.dhost)

		assert.Check(t, is.DeepEqual(actual, tc.expected))
	}
}

func testRegistryHost(scheme, host, path string, caps docker.HostCapabilities) docker.RegistryHost {
	return docker.RegistryHost{
		Host:         host,
		Scheme:       scheme,
		Path:         path,
		Capabilities: caps,
	}
}
