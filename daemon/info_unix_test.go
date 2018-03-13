// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestParseInitVersion(t *testing.T) {
	tests := []struct {
		version string
		result  types.Commit
		invalid bool
	}{
		{
			version: "tini version 0.13.0 - git.949e6fa",
			result:  types.Commit{ID: "949e6fa", Expected: dockerversion.InitCommitID[0:7]},
		}, {
			version: "tini version 0.13.0\n",
			result:  types.Commit{ID: "v0.13.0", Expected: dockerversion.InitCommitID},
		}, {
			version: "tini version 0.13.2",
			result:  types.Commit{ID: "v0.13.2", Expected: dockerversion.InitCommitID},
		}, {
			version: "tini version0.13.2",
			result:  types.Commit{ID: "N/A", Expected: dockerversion.InitCommitID},
			invalid: true,
		}, {
			version: "",
			result:  types.Commit{ID: "N/A", Expected: dockerversion.InitCommitID},
			invalid: true,
		}, {
			version: "hello world",
			result:  types.Commit{ID: "N/A", Expected: dockerversion.InitCommitID},
			invalid: true,
		},
	}

	for _, test := range tests {
		ver, err := parseInitVersion(string(test.version))
		if test.invalid {
			assert.Check(t, is.ErrorContains(err, ""))
		} else {
			assert.Check(t, err)
		}
		assert.Check(t, is.DeepEqual(test.result, ver))
	}
}
