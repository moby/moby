package config

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

func TestBuilderGC(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{
  "builder": {
    "gc": {
      "enabled": true,
      "policy": [
        {"keepStorage": "10GB", "filter": ["unused-for=2200h"]},
        {"keepStorage": "50GB", "filter": {"unused-for": {"3300h": true}}},
        {"keepStorage": "100GB", "all": true}
      ]
    }
  }
}`))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	cfg, err := MergeDaemonConfigurations(&Config{}, nil, configFile)
	assert.NilError(t, err)
	assert.Assert(t, cfg.Builder.GC.Enabled)
	f1 := filters.NewArgs()
	f1.Add("unused-for", "2200h")
	f2 := filters.NewArgs()
	f2.Add("unused-for", "3300h")
	expectedPolicy := []BuilderGCRule{
		{KeepStorage: "10GB", Filter: BuilderGCFilter(f1)},
		{KeepStorage: "50GB", Filter: BuilderGCFilter(f2)}, /* parsed from deprecated form */
		{KeepStorage: "100GB", All: true},
	}
	assert.DeepEqual(t, cfg.Builder.GC.Policy, expectedPolicy, cmp.AllowUnexported(BuilderGCFilter{}))
	// double check to please the skeptics
	assert.Assert(t, filters.Args(cfg.Builder.GC.Policy[0].Filter).UniqueExactMatch("unused-for", "2200h"))
	assert.Assert(t, filters.Args(cfg.Builder.GC.Policy[1].Filter).UniqueExactMatch("unused-for", "3300h"))
}

// TestBuilderGCFilterUnmarshal is a regression test for https://github.com/moby/moby/issues/44361,
// where and incorrectly formatted gc filter option ("unused-for2200h",
// missing a "=" separator). resulted in a panic during unmarshal.
func TestBuilderGCFilterUnmarshal(t *testing.T) {
	var cfg BuilderGCConfig
	err := json.Unmarshal([]byte(`{"poliCy": [{"keepStorage": "10GB", "filter": ["unused-for2200h"]}]}`), &cfg)
	assert.Check(t, err)
	expectedPolicy := []BuilderGCRule{{
		KeepStorage: "10GB", Filter: BuilderGCFilter(filters.NewArgs(filters.Arg("unused-for2200h", ""))),
	}}
	assert.DeepEqual(t, cfg.Policy, expectedPolicy, cmp.AllowUnexported(BuilderGCFilter{}))
}
