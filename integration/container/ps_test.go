package container // import "github.com/docker/docker/integration/container"

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestPsFilter(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prev := container.Create(ctx, t, apiClient)
	top := container.Create(ctx, t, apiClient)
	next := container.Create(ctx, t, apiClient)

	containerIDs := func(containers []containertypes.Summary) []string {
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

// TestPsPlatform verifies that containers have a platform set
func TestPsImageManifestPlatform(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !testEnv.UsingSnapshotter())

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	container.Create(ctx, t, apiClient)

	containers, err := apiClient.ContainerList(ctx, containertypes.ListOptions{
		All: true,
	})
	assert.NilError(t, err)
	assert.Check(t, len(containers) > 0)

	ctr := containers[0]
	if assert.Check(t, ctr.ImageManifestDescriptor != nil && ctr.ImageManifestDescriptor.Platform != nil) {
		// Check that at least OS and Architecture have a value. Other values
		// depend on the platform on which we're running the test.
		assert.Equal(t, ctr.ImageManifestDescriptor.Platform.OS, testEnv.DaemonInfo.OSType)
		assert.Check(t, ctr.ImageManifestDescriptor.Platform.Architecture != "")
	}
}
