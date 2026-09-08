package daemon

import (
	"testing"

	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/jobs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// testDaemon builds a daemon carrying only the given config: registration
// never dereferences anything else, the optional builders run lazily and
// only for known IDs.
func testDaemon(cfg *config.Config) *Daemon {
	d := &Daemon{}
	d.configStore.Store(&configStore{Config: *cfg})
	return d
}

func TestDaemonExtensionsEnableExtensions(t *testing.T) {
	// The always-on builtins, whatever their number grows to: an empty list
	// must add nothing on top of them.
	alwaysOn, err := testDaemon(&config.Config{}).daemonExtensions()
	assert.NilError(t, err)

	tests := []struct {
		doc       string
		enable    []string
		wantExtra int
		wantErr   string
	}{
		{
			doc:       "jobs opts in by extension ID",
			enable:    []string{jobs.ExtensionID},
			wantExtra: 1,
		},
		{
			doc:       "duplicate IDs register the extension once",
			enable:    []string{jobs.ExtensionID, jobs.ExtensionID},
			wantExtra: 1,
		},
		{
			doc:     "an unknown ID fails startup loudly",
			enable:  []string{"org.example.bogus.v1"},
			wantErr: `unknown built-in extension "org.example.bogus.v1"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.EnableExtensions = tc.enable
			exts, err := testDaemon(cfg).daemonExtensions()
			if tc.wantErr != "" {
				assert.Check(t, is.ErrorContains(err, tc.wantErr))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, is.Len(exts, len(alwaysOn)+tc.wantExtra))
		})
	}
}
