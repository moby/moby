package network

import (
	"runtime"
	"testing"

	"github.com/moby/moby/api/types/network"
)

func TestIsPreDefined(t *testing.T) {
	tests := []struct {
		name     string
		network  string
		expected bool
		skipOn   string
	}{
		// Predefined networks
		{
			name:     "none network",
			network:  network.NetworkNone,
			expected: true,
		},
		{
			name:     "default network",
			network:  network.NetworkDefault,
			expected: true,
		},
		// User-defined networks
		{
			name:     "custom network name",
			network:  "mynetwork",
			expected: false,
		},
		{
			name:     "custom network with special chars",
			network:  "my-custom_network.123",
			expected: false,
		},
		// Edge cases
		{
			name:     "empty string",
			network:  "",
			expected: false, // empty string is not a predefined network name
		},
		{
			name:     "containerX (not container mode)",
			network:  "containerX",
			expected: false,
		},
		{
			name:     "Xcontainer (not container mode)",
			network:  "Xcontainer",
			expected: false,
		},
		// Platform-specific tests
		{
			name:     "bridge network",
			network:  network.NetworkBridge,
			expected: true,
			skipOn:   "windows",
		},
		{
			name:     "host network",
			network:  network.NetworkHost,
			expected: true,
			skipOn:   "windows",
		},
	}

	// Windows-specific tests
	if runtime.GOOS == "windows" {
		tests = append(tests, []struct {
			name     string
			network  string
			expected bool
			skipOn   string
		}{
			{
				name:     "nat network",
				network:  network.NetworkNat,
				expected: true,
			},
			{
				name:     "bridge network (Windows - should be false)",
				network:  network.NetworkBridge,
				expected: false,
			},
		}...)
	}

	for _, tc := range tests {
		if tc.skipOn == runtime.GOOS {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			result := isPreDefined(tc.network)
			if result != tc.expected {
				t.Errorf("isPreDefined(%q) = %v, expected %v", tc.network, result, tc.expected)
			}
		})
	}
}

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name     string
		network  string
		expected bool
	}{
		// Reserved network names from issue #51949
		{
			name:     "container without colon",
			network:  "container",
			expected: true,
		},
		{
			name:     "container with empty ID",
			network:  "container:",
			expected: true,
		},
		{
			name:     "container with valid ID",
			network:  "container:abc123",
			expected: true, // reserved to prevent confusion with container mode
		},
		{
			name:     "container with full container ID",
			network:  "container:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: true, // reserved to prevent confusion with container mode
		},
		// Predefined networks should not be considered reserved
		{
			name:     "none network",
			network:  network.NetworkNone,
			expected: false,
		},
		{
			name:     "default network",
			network:  network.NetworkDefault,
			expected: false,
		},
		{
			name:     "bridge network",
			network:  network.NetworkBridge,
			expected: false,
		},
		{
			name:     "host network",
			network:  network.NetworkHost,
			expected: false,
		},
		// User-defined networks
		{
			name:     "custom network name",
			network:  "mynetwork",
			expected: false,
		},
		{
			name:     "custom network with special chars",
			network:  "my-custom_network.123",
			expected: false,
		},
		// Edge cases
		{
			name:     "empty string",
			network:  "",
			expected: false,
		},
		{
			name:     "containerX (not container mode)",
			network:  "containerX",
			expected: false,
		},
		{
			name:     "Xcontainer (not container mode)",
			network:  "Xcontainer",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsReserved(tc.network)
			if result != tc.expected {
				t.Errorf("IsReserved(%q) = %v, expected %v", tc.network, result, tc.expected)
			}
		})
	}
}
