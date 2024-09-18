package container

import (
	"fmt"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestListAnnotations(t *testing.T) {
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
