package daemon

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

// ActiveContainers returns the list of ids of the currently running containers
func (d *Daemon) ActiveContainers(ctx context.Context, t testing.TB) []string {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
	assert.NilError(t, err)

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	return ids
}
