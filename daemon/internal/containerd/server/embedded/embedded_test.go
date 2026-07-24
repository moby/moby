//go:build (linux || windows) && !no_embedded_containerd

package embedded

import (
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestPluginGraphResolves guards the blank-import plugin set: it starts the
// embedded containerd server and fails if any required plugin is missing from
// the registry or its dependency graph cannot be satisfied. Runtime init
// failures unrelated to registration (e.g. requiring root) are tolerated, so
// the test is safe to run unprivileged.
func TestPluginGraphResolves(t *testing.T) {
	ctx := t.Context()
	d, err := Start(ctx, t.TempDir(), t.TempDir())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no plugins registered") {
			t.Fatalf("embedded containerd plugin graph is incomplete: %v", err)
		}
		t.Skipf("embedded containerd did not start in this environment: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(ctx) })
}

func TestBuildServerConfigUsesSupervisedLayout(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "root")
	stateDir := filepath.Join(t.TempDir(), "state")
	address := defaultAddress(stateDir)

	cfg := buildServerConfig(rootDir, stateDir, address)

	assert.Check(t, is.Equal(cfg.root, rootDir))
	assert.Check(t, is.Equal(cfg.state, filepath.Join(stateDir, "daemon")))
	assert.Check(t, is.Equal(cfg.grpcAddress, address))
	assert.Check(t, is.Equal(cfg.ttrpcAddress, address+".ttrpc"))
}
