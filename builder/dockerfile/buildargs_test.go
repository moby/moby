package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"bytes"
	"strings"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func strPtr(source string) *string {
	return &source
}

func TestGetAllAllowed(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ArgNotUsedInDockerfile":              strPtr("fromopt1"),
		"ArgOverriddenByOptions":              strPtr("fromopt2"),
		"ArgNoDefaultInDockerfileFromOptions": strPtr("fromopt3"),
		"HTTP_PROXY":                          strPtr("theproxy"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", strPtr("frommeta1"))
	buildArgs.AddMetaArg("ArgFromMetaOverridden", strPtr("frommeta2"))
	buildArgs.AddMetaArg("ArgFromMetaNotUsed", strPtr("frommeta3"))

	buildArgs.AddArg("ArgOverriddenByOptions", strPtr("fromdockerfile2"))
	buildArgs.AddArg("ArgWithDefaultInDockerfile", strPtr("fromdockerfile1"))
	buildArgs.AddArg("ArgNoDefaultInDockerfile", nil)
	buildArgs.AddArg("ArgNoDefaultInDockerfileFromOptions", nil)
	buildArgs.AddArg("ArgFromMeta", nil)
	buildArgs.AddArg("ArgFromMetaOverridden", strPtr("fromdockerfile3"))

	all := buildArgs.GetAllAllowed()
	expected := map[string]string{
		"HTTP_PROXY":                          "theproxy",
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
		"ArgNotUsedInDockerfile":        strPtr("fromopt1"),
		"ArgOverriddenByOptions":        strPtr("fromopt2"),
		"ArgNoDefaultInMetaFromOptions": strPtr("fromopt3"),
		"HTTP_PROXY":                    strPtr("theproxy"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", strPtr("frommeta1"))
	buildArgs.AddMetaArg("ArgOverriddenByOptions", strPtr("frommeta2"))
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
		"ThisArgIsUsed":    strPtr("fromopt1"),
		"ThisArgIsNotUsed": strPtr("fromopt2"),
		"HTTPS_PROXY":      strPtr("referenced builtin"),
		"HTTP_PROXY":       strPtr("unreferenced builtin"),
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
		"ThisArgIsUsed":    strPtr("fromopt1"),
		"ThisArgIsNotUsed": strPtr("fromopt2"),
		"HTTPS_PROXY":      strPtr("referenced builtin"),
		"HTTP_PROXY":       strPtr("unreferenced builtin"),
	})
	buildArgs.AddArg("ThisArgIsUsed", nil)
	buildArgs.AddArg("HTTPS_PROXY", nil)

	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("ThisArgIsUsed"))
	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("ThisArgIsNotUsed"))
	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("HTTPS_PROXY"))
	assert.Check(t, !buildArgs.IsReferencedOrNotBuiltin("HTTP_PROXY"))
}
