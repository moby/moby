package daemon

import (
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMirrorsToHosts(t *testing.T) {
	pullCaps := docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityReferrers
	allCaps := docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush | docker.HostCapabilityReferrers
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

func TestMirrorsToHosts_DistinctHostFields(t *testing.T) {
	// Verifies that mirror hosts have distinct Host fields from the primary,
	// which is critical for hostsWrapper to apply credentials correctly.
	primaryHost := "registry-1.docker.io"
	allCaps := docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush | docker.HostCapabilityReferrers
	dHost := testRegistryHost("https", primaryHost, "/v2", allCaps)

	mirrors := []string{
		"https://nexus.example.com:5000",
		"https://harbor.corp.internal",
	}

	hosts := mirrorsToRegistryHosts(mirrors, dHost)
	assert.Assert(t, is.Len(hosts, 3)) // 2 mirrors + 1 primary

	// Each mirror must have a Host value different from the primary.
	for i, h := range hosts[:len(hosts)-1] {
		assert.Check(t, h.Host != primaryHost,
			"mirror %d has same Host as primary (%s); hostsWrapper cannot distinguish them", i, primaryHost)
	}

	// The last entry must be the primary.
	assert.Check(t, is.Equal(hosts[len(hosts)-1].Host, primaryHost))
}

func testRegistryHost(scheme, host, path string, caps docker.HostCapabilities) docker.RegistryHost {
	return docker.RegistryHost{
		Host:         host,
		Scheme:       scheme,
		Path:         path,
		Capabilities: caps,
	}
}
