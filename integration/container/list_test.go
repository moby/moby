package container

import (
	"fmt"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestContainerList(t *testing.T) {
	ctx := setupTest(t)
	apiClient := request.NewAPIClient(t)

	// remove all existing containers
	container.RemoveAll(ctx, t, apiClient)

	// create the containers
	num := 64
	containers := make([]string, num)
	for i := range num {
		id := container.Create(ctx, t, apiClient)
		defer container.Remove(ctx, t, apiClient, id, containertypes.RemoveOptions{Force: true})
		containers[i] = id
	}

	// list them and verify correctness
	containerList, err := apiClient.ContainerList(ctx, containertypes.ListOptions{All: true})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerList, num))
	for i := range num {
		// container list should be ordered in descending creation order
		assert.Assert(t, is.Equal(containerList[i].ID, containers[num-1-i]))
	}
}

func TestContainerList_Annotations(t *testing.T) {
	ctx := setupTest(t)

	annotations := map[string]string{
		"foo":                       "bar",
		"io.kubernetes.docker.type": "container",
	}
	testcases := []struct {
		apiVersion          string
		expectedAnnotations map[string]string
	}{
		{apiVersion: "1.44", expectedAnnotations: nil},
		{apiVersion: "1.46", expectedAnnotations: annotations},
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("run with version v%s", tc.apiVersion), func(t *testing.T) {
			apiClient := request.NewAPIClient(t, client.WithVersion(tc.apiVersion))
			id := container.Create(ctx, t, apiClient, container.WithAnnotations(annotations))
			defer container.Remove(ctx, t, apiClient, id, containertypes.RemoveOptions{Force: true})

			containers, err := apiClient.ContainerList(ctx, containertypes.ListOptions{
				All:     true,
				Filters: filters.NewArgs(filters.Arg("id", id)),
			})
			assert.NilError(t, err)
			assert.Assert(t, is.Len(containers, 1))
			assert.Equal(t, containers[0].ID, id)
			assert.Check(t, is.DeepEqual(containers[0].HostConfig.Annotations, tc.expectedAnnotations))
		})
	}
}

func TestContainerList_Filter(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prev := container.Create(ctx, t, apiClient)
	top := container.Create(ctx, t, apiClient)
	next := container.Create(ctx, t, apiClient)

	defer func() {
		container.Remove(ctx, t, apiClient, prev, containertypes.RemoveOptions{Force: true})
		container.Remove(ctx, t, apiClient, top, containertypes.RemoveOptions{Force: true})
		container.Remove(ctx, t, apiClient, next, containertypes.RemoveOptions{Force: true})
	}()

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

// TestListPlatform verifies that containers have a platform set
func TestContainerList_ImageManifestPlatform(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !testEnv.UsingSnapshotter())

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	id := container.Create(ctx, t, apiClient)
	defer container.Remove(ctx, t, apiClient, id, containertypes.RemoveOptions{Force: true})

	containers, err := apiClient.ContainerList(ctx, containertypes.ListOptions{
		All: true,
	})
	assert.NilError(t, err)
	assert.Assert(t, len(containers) > 0)

	ctr := containers[0]
	if assert.Check(t, ctr.ImageManifestDescriptor != nil && ctr.ImageManifestDescriptor.Platform != nil) {
		// Check that at least OS and Architecture have a value. Other values
		// depend on the platform on which we're running the test.
		assert.Equal(t, ctr.ImageManifestDescriptor.Platform.OS, testEnv.DaemonInfo.OSType)
		assert.Check(t, ctr.ImageManifestDescriptor.Platform.Architecture != "")
	}
}
