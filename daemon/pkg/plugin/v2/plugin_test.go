package v2

import (
	"path/filepath"
	"strings"
	"testing"

	plugintypes "github.com/moby/moby/api/types/plugin"
)

func TestScopedPath(t *testing.T) {
	const (
		pluginID = "abc123"
		rootfs   = "/var/lib/docker/plugins/" + pluginID + "/rootfs"
		propRoot = "/var/lib/docker/plugins/" + pluginID + "/propagated-mount"
	)

	newPlugin := func(propagatedMount, rootfs string) *Plugin {
		return &Plugin{
			Rootfs: rootfs,
			PluginObj: plugintypes.Plugin{
				Config: plugintypes.Config{
					PropagatedMount: propagatedMount,
				},
			},
		}
	}

	t.Run("normal path within propagated mount", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		got := p.ScopedPath("/propagated/vol/_data")
		want := propRoot + "/vol/_data"
		if got != want {
			t.Errorf("ScopedPath() = %q, want %q", got, want)
		}
	})

	t.Run("path traversal escape via ../", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		got := p.ScopedPath("/propagated/../../../etc/passwd")
		// After cleaning, the escaped path should fall back to propRoot
		if !strings.HasPrefix(filepath.Clean(got), propRoot) {
			t.Errorf("ScopedPath() = %q, cleaned=%q, should start with %q", got, filepath.Clean(got), propRoot)
		}
		if got != propRoot {
			t.Errorf("ScopedPath() = %q, want fallback %q", got, propRoot)
		}
	})

	t.Run("path traversal with exact prefix then ../", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		got := p.ScopedPath("/propagated/../rootfs/etc")
		if !strings.HasPrefix(filepath.Clean(got), propRoot) {
			t.Errorf("ScopedPath() = %q, cleaned=%q, should start with %q", got, filepath.Clean(got), propRoot)
		}
		if got != propRoot {
			t.Errorf("ScopedPath() = %q, want fallback %q", got, propRoot)
		}
	})

	t.Run("traversal with many ../", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		got := p.ScopedPath("/propagated/../../../../../../../../../tmp/evil")
		if !strings.HasPrefix(filepath.Clean(got), propRoot) {
			t.Errorf("ScopedPath() = %q, cleaned=%q, should start with %q", got, filepath.Clean(got), propRoot)
		}
		if got != propRoot {
			t.Errorf("ScopedPath() = %q, want fallback %q", got, propRoot)
		}
	})

	t.Run("empty propagated mount falls through to rootfs", func(t *testing.T) {
		p := newPlugin("", rootfs)
		got := p.ScopedPath("/some/path")
		want := filepath.Join(rootfs, "/some/path")
		if got != want {
			t.Errorf("ScopedPath() = %q, want %q", got, want)
		}
	})

	t.Run("non-prefix path ignores propagated mount", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		// Path doesn't start with PropagatedMount, so should go to rootfs
		got := p.ScopedPath("/other/path")
		want := filepath.Join(rootfs, "/other/path")
		if got != want {
			t.Errorf("ScopedPath() = %q, want %q", got, want)
		}
	})

	t.Run("exact propagated mount prefix match returns propRoot", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		got := p.ScopedPath("/propagated")
		want := propRoot
		if got != want {
			t.Errorf("ScopedPath() = %q, want %q", got, want)
		}
	})

	t.Run("deeply nested normal path unaffected", func(t *testing.T) {
		p := newPlugin("/propagated", rootfs)
		got := p.ScopedPath("/propagated/a/b/c/d/e/f/_data")
		want := propRoot + "/a/b/c/d/e/f/_data"
		if got != want {
			t.Errorf("ScopedPath() = %q, want %q", got, want)
		}
	})
}
