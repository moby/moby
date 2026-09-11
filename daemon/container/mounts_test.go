package container_test

import (
	"testing"

	"github.com/moby/moby/v2/daemon/container"
	"gotest.tools/v3/assert"
)

// TestSortMounts verifies that mounts are sorted by destination depth and
// deterministically by destination when they have the same depth.
func TestSortMounts(t *testing.T) {
	tests := []struct {
		name     string
		mounts   []container.Mount
		expected []container.Mount
	}{
		{
			name: "nested",
			mounts: []container.Mount{
				{Destination: "/etc/resolv.conf"},
				{Destination: "/etc"},
			},
			expected: []container.Mount{
				{Destination: "/etc"},
				{Destination: "/etc/resolv.conf"},
			},
		},
		{
			name: "same depth",
			mounts: []container.Mount{
				{Destination: "/var"},
				{Destination: "/etc"},
				{Destination: "/usr"},
			},
			expected: []container.Mount{
				{Destination: "/etc"},
				{Destination: "/usr"},
				{Destination: "/var"},
			},
		},
		{
			name: "nested and same depth",
			mounts: []container.Mount{
				{Destination: "/var/lib/foo"},
				{Destination: "/etc/resolv.conf"},
				{Destination: "/var"},
				{Destination: "/etc"},
				{Destination: "/opt/data"},
			},
			expected: []container.Mount{
				{Destination: "/etc"},
				{Destination: "/var"},
				{Destination: "/etc/resolv.conf"},
				{Destination: "/opt/data"},
				{Destination: "/var/lib/foo"},
			},
		},
		{
			name: "duplicate destinations preserve order",
			mounts: []container.Mount{
				{Source: "/source-1", Destination: "/var"},
				{Source: "/source-c", Destination: "/etc"},
				{Source: "/source-2", Destination: "/opt"},
				{Source: "/source-a", Destination: "/etc"},
				{Source: "/source-3", Destination: "/usr"},
				{Source: "/source-b", Destination: "/etc"},
				{Source: "/source-4", Destination: "/home"},
			},
			expected: []container.Mount{
				{Source: "/source-c", Destination: "/etc"},
				{Source: "/source-a", Destination: "/etc"},
				{Source: "/source-b", Destination: "/etc"},
				{Source: "/source-4", Destination: "/home"},
				{Source: "/source-2", Destination: "/opt"},
				{Source: "/source-3", Destination: "/usr"},
				{Source: "/source-1", Destination: "/var"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			container.SortMounts(tc.mounts)
			assert.DeepEqual(t, tc.expected, tc.mounts)
		})
	}
}
