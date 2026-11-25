//go:build linux && !no_embedded_containerd

package embedded

import (
	"testing"

	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/plugin"
)

func TestPluginRegistrationsIncludeErofs(t *testing.T) {
	registered := make(map[plugin.Type]bool)
	cfg := &serverConfig{}
	for _, registration := range pluginRegistrations(cfg) {
		if registration.ID == "erofs" {
			registered[registration.Type] = true
		}
	}

	if !registered[plugins.SnapshotPlugin] {
		t.Fatal("EROFS snapshotter plugin is not registered")
	}
	if !registered[plugins.DiffPlugin] {
		t.Fatal("EROFS differ plugin is not registered")
	}
}
