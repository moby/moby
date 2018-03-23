package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestPsFilter(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	prev := container.Create(t, ctx, client, container.WithName("prev-"+t.Name()))
	topContainerName := "top-" + t.Name()
	container.Create(t, ctx, client, container.WithName(topContainerName))
	next := container.Create(t, ctx, client, container.WithName("next-"+t.Name()))

	containerIDs := func(containers []types.Container) []string {
		entries := []string{}
		for _, container := range containers {
			entries = append(entries, container.ID)
		}
		return entries
	}

	f1 := filters.NewArgs()
	f1.Add("since", topContainerName)
	q1, err := client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: f1,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(containerIDs(q1), next))

	f2 := filters.NewArgs()
	f2.Add("before", topContainerName)
	q2, err := client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: f2,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(containerIDs(q2), prev))
}
