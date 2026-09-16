package v2

import (
	"testing"

	"github.com/moby/moby/api/types/plugin"
	"gotest.tools/v3/assert"
)

func TestScopedPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		propagatedMount string
		path            string
		expected        string
	}{
		{
			name:            "ordinary rootfs path",
			propagatedMount: "/propagated",
			path:            "/etc/plugin.json",
			expected:        "/plugin/rootfs/etc/plugin.json",
		},
		{
			name:            "exact propagated root",
			propagatedMount: "/propagated",
			path:            "/propagated",
			expected:        "/plugin/propagated-mount",
		},
		{
			name:            "propagated child",
			propagatedMount: "/propagated",
			path:            "/propagated/child",
			expected:        "/plugin/propagated-mount/child",
		},
		{
			name:            "in-subtree normalization",
			propagatedMount: "/propagated",
			path:            "/propagated/subtree/../child",
			expected:        "/plugin/propagated-mount/child",
		},
		{
			name:            "reported traversal escape",
			propagatedMount: "/propagated",
			path:            "/propagated/../../../volumes/victim/_data",
			expected:        "/plugin/rootfs/volumes/victim/_data",
		},
		{
			name:            "sibling prefix path",
			propagatedMount: "/propagated",
			path:            "/propagated-other/child",
			expected:        "/plugin/rootfs/propagated-other/child",
		},
		{
			name:            "relative dot-dot containment",
			propagatedMount: "/propagated",
			path:            "../../../outside",
			expected:        "/plugin/rootfs/outside",
		},
		{
			name:            "trailing separator",
			propagatedMount: "/propagated/",
			path:            "/propagated/",
			expected:        "/plugin/propagated-mount",
		},
		{
			name:            "propagated root slash",
			propagatedMount: "/",
			path:            "/var/lib",
			expected:        "/plugin/propagated-mount/var/lib",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := &Plugin{
				Rootfs: "/plugin/rootfs",
				PluginObj: plugin.Plugin{
					Config: plugin.Config{PropagatedMount: tc.propagatedMount},
				},
			}
			assert.Equal(t, p.ScopedPath(tc.path), tc.expected)
		})
	}
}
