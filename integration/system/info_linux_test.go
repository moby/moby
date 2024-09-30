//go:build !windows

package system // import "github.com/docker/docker/integration/system"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoBinaryCommits(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	info, err := apiClient.Info(ctx)
	assert.NilError(t, err)

	assert.Check(t, "N/A" != info.ContainerdCommit.ID)
	assert.Check(t, is.Equal(info.ContainerdCommit.Expected, info.ContainerdCommit.ID)) //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.49.

	assert.Check(t, "N/A" != info.InitCommit.ID)
	assert.Check(t, is.Equal(info.InitCommit.Expected, info.InitCommit.ID)) //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.49.

	assert.Check(t, "N/A" != info.RuncCommit.ID)
	assert.Check(t, is.Equal(info.RuncCommit.Expected, info.RuncCommit.ID)) //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.49.
}
