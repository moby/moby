package container

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/client"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// GetContainerNS gets the value of the specified namespace of a container
func GetContainerNS(ctx context.Context, t *testing.T, apiClient client.APIClient, cID, nsName string) string {
	t.Helper()
	res, err := Exec(ctx, apiClient, cID, []string{"readlink", "/proc/self/ns/" + nsName})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	return strings.TrimSpace(res.Stdout())
}
