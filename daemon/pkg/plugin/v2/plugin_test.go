package v2

import (
	"path/filepath"
	"testing"

	"github.com/moby/moby/api/types/plugin"
)

func TestScopedPath(t *testing.T) {
	rootfs := "/var/lib/docker/plugins/abc123/rootfs"
	propagatedMount := "/propagated"

	tests := []struct {
		name            string
		propagatedMount string
		input           string
		want            string
	}{
		{
			name:  "no propagated mount, plain path",
			input: "/some/path",
			want:  filepath.Join(rootfs, "/some/path"),
		},
		{
			name:            "path within propagated mount",
			propagatedMount: propagatedMount,
			input:           "/propagated/data",
			want:            filepath.Join(filepath.Dir(rootfs), "propagated-mount", "/data"),
		},
		{
			name:            "path outside propagated mount",
			propagatedMount: propagatedMount,
			input:           "/other/path",
			want:            filepath.Join(rootfs, "/other/path"),
		},
		{
			name:            "traversal attempt via propagated mount prefix",
			propagatedMount: propagatedMount,
			input:           "/propagated/../../../volumes/victimvol/_data",
			want:            filepath.Join(rootfs, "/volumes/victimvol/_data"),
		},
		{
			name:            "traversal with double-dot stays within propagated mount",
			propagatedMount: propagatedMount,
			input:           "/propagated/sub/../data",
			want:            filepath.Join(filepath.Dir(rootfs), "propagated-mount", "/data"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{
				PluginObj: plugin.Plugin{
					Config: plugin.Config{
						PropagatedMount: tc.propagatedMount,
					},
				},
				Rootfs: rootfs,
			}
			got := p.ScopedPath(tc.input)
			if got != tc.want {
				t.Errorf("ScopedPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
