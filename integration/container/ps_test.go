package container // import "github.com/docker/docker/integration/container"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
)

func TestPsFilter(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prev := container.Create(ctx, t, apiClient)
	top := container.Create(ctx, t, apiClient)
	next := container.Create(ctx, t, apiClient)

	containerIDs := func(containers []types.Container) []string {
		var entries []string
		for _, c := range containers {
			entries = append(entries, c.ID)
		}
		return entries
	}

	t.Run("since", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		results, err := apiClient.ContainerList(ctx, containertypes.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("since", top)),
		})
		assert.NilError(t, err)
		assert.Check(t, is.Contains(containerIDs(results), next))
	})

	t.Run("before", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		results, err := apiClient.ContainerList(ctx, containertypes.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("before", top)),
		})
		assert.NilError(t, err)
		assert.Check(t, is.Contains(containerIDs(results), prev))
	})
}
