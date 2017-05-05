package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func strPtr(source string) *string {
	return &source
}

func TestGetAllAllowed(t *testing.T) {
	buildArgs := newBuildArgs(map[string]*string{
		"ArgNotUsedInDockerfile":              strPtr("fromopt1"),
		"ArgOverriddenByOptions":              strPtr("fromopt2"),
		"ArgNoDefaultInDockerfileFromOptions": strPtr("fromopt3"),
		"HTTP_PROXY":                          strPtr("theproxy"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", strPtr("frommeta1"))
	buildArgs.AddMetaArg("ArgFromMetaOverriden", strPtr("frommeta2"))
	buildArgs.AddMetaArg("ArgFromMetaNotUsed", strPtr("frommeta3"))

	buildArgs.AddArg("ArgOverriddenByOptions", strPtr("fromdockerfile2"))
	buildArgs.AddArg("ArgWithDefaultInDockerfile", strPtr("fromdockerfile1"))
	buildArgs.AddArg("ArgNoDefaultInDockerfile", nil)
	buildArgs.AddArg("ArgNoDefaultInDockerfileFromOptions", nil)
	buildArgs.AddArg("ArgFromMeta", nil)
	buildArgs.AddArg("ArgFromMetaOverriden", strPtr("fromdockerfile3"))

	all := buildArgs.GetAllAllowed()
	expected := map[string]string{
		"HTTP_PROXY":                          "theproxy",
		"ArgOverriddenByOptions":              "fromopt2",
		"ArgWithDefaultInDockerfile":          "fromdockerfile1",
		"ArgNoDefaultInDockerfileFromOptions": "fromopt3",
		"ArgFromMeta":                         "frommeta1",
		"ArgFromMetaOverriden":                "fromdockerfile3",
	}
	assert.Equal(t, expected, all)
}

func TestGetAllMeta(t *testing.T) {
	buildArgs := newBuildArgs(map[string]*string{
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
	assert.Equal(t, expected, all)
}
