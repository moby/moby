package daemon

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"gotest.tools/v3/assert"
)

// ActiveContainers returns the list of ids of the currently running containers
func (d *Daemon) ActiveContainers(ctx context.Context, tb testing.TB) []string {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), container.ListOptions{})
	assert.NilError(tb, err)

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	return ids
}

// FindContainerIP returns the ip of the specified container
func (d *Daemon) FindContainerIP(tb testing.TB, id string) string {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	i, err := cli.ContainerInspect(context.Background(), id)
	assert.NilError(tb, err)
	return i.NetworkSettings.IPAddress
}
