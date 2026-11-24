//go:build linux && !no_embedded_containerd

package embedded

import (
	"testing"

	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
)

func TestErofsPluginsRegistered(t *testing.T) {
	registered := make(map[plugin.Type]bool)
	for _, p := range registry.Graph(nil) {
		if p.ID == "erofs" {
			registered[p.Type] = true
		}
	}

	if !registered[plugins.SnapshotPlugin] {
		t.Fatal("EROFS snapshotter plugin is not registered")
	}
	if !registered[plugins.DiffPlugin] {
		t.Fatal("EROFS differ plugin is not registered")
	}
}
