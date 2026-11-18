package dockerfile

import (
	"bytes"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGetAllAllowed(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ArgNotUsedInDockerfile":              new("fromopt1"),
		"ArgOverriddenByOptions":              new("fromopt2"),
		"ArgNoDefaultInDockerfileFromOptions": new("fromopt3"),
		"HTTP_PROXY":                          new("theproxy"),
		"all_proxy":                           new("theproxy2"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", new("frommeta1"))
	buildArgs.AddMetaArg("ArgFromMetaOverridden", new("frommeta2"))
	buildArgs.AddMetaArg("ArgFromMetaNotUsed", new("frommeta3"))

	buildArgs.AddArg("ArgOverriddenByOptions", new("fromdockerfile2"))
	buildArgs.AddArg("ArgWithDefaultInDockerfile", new("fromdockerfile1"))
	buildArgs.AddArg("ArgNoDefaultInDockerfile", nil)
	buildArgs.AddArg("ArgNoDefaultInDockerfileFromOptions", nil)
	buildArgs.AddArg("ArgFromMeta", nil)
	buildArgs.AddArg("ArgFromMetaOverridden", new("fromdockerfile3"))

	all := buildArgs.GetAllAllowed()
	expected := map[string]string{
		"HTTP_PROXY":                          "theproxy",
		"all_proxy":                           "theproxy2",
		"ArgOverriddenByOptions":              "fromopt2",
		"ArgWithDefaultInDockerfile":          "fromdockerfile1",
		"ArgNoDefaultInDockerfileFromOptions": "fromopt3",
		"ArgFromMeta":                         "frommeta1",
		"ArgFromMetaOverridden":               "fromdockerfile3",
	}
	assert.Check(t, is.DeepEqual(expected, all))
}

func TestGetAllMeta(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ArgNotUsedInDockerfile":        new("fromopt1"),
		"ArgOverriddenByOptions":        new("fromopt2"),
		"ArgNoDefaultInMetaFromOptions": new("fromopt3"),
		"HTTP_PROXY":                    new("theproxy"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", new("frommeta1"))
	buildArgs.AddMetaArg("ArgOverriddenByOptions", new("frommeta2"))
	buildArgs.AddMetaArg("ArgNoDefaultInMetaFromOptions", nil)

	all := buildArgs.GetAllMeta()
	expected := map[string]string{
		"HTTP_PROXY":                    "theproxy",
		"ArgFromMeta":                   "frommeta1",
		"ArgOverriddenByOptions":        "fromopt2",
		"ArgNoDefaultInMetaFromOptions": "fromopt3",
	}
	assert.Check(t, is.DeepEqual(expected, all))
}

func TestWarnOnUnusedBuildArgs(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ThisArgIsUsed":    new("fromopt1"),
		"ThisArgIsNotUsed": new("fromopt2"),
		"HTTPS_PROXY":      new("referenced builtin"),
		"HTTP_PROXY":       new("unreferenced builtin"),
	})
	buildArgs.AddArg("ThisArgIsUsed", nil)
	buildArgs.AddArg("HTTPS_PROXY", nil)

	buffer := new(bytes.Buffer)
	buildArgs.WarnOnUnusedBuildArgs(buffer)
	out := buffer.String()
	assert.Assert(t, !strings.Contains(out, "ThisArgIsUsed"), out)
	assert.Assert(t, !strings.Contains(out, "HTTPS_PROXY"), out)
	assert.Assert(t, !strings.Contains(out, "HTTP_PROXY"), out)
	assert.Check(t, is.Contains(out, "ThisArgIsNotUsed"))
}

func TestIsUnreferencedBuiltin(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ThisArgIsUsed":    new("fromopt1"),
		"ThisArgIsNotUsed": new("fromopt2"),
		"HTTPS_PROXY":      new("referenced builtin"),
		"HTTP_PROXY":       new("unreferenced builtin"),
	})
	buildArgs.AddArg("ThisArgIsUsed", nil)
	buildArgs.AddArg("HTTPS_PROXY", nil)

	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("ThisArgIsUsed"))
	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("ThisArgIsNotUsed"))
	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("HTTPS_PROXY"))
	assert.Check(t, !buildArgs.IsReferencedOrNotBuiltin("HTTP_PROXY"))
}
