package daemon

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/gotestyourself/gotestyourself/assert"
)

// ActiveContainers returns the list of ids of the currently running containers
func (d *Daemon) ActiveContainers(t testingT) []string {
	cli := d.NewClientT(t)
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	assert.NilError(t, err)

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	return ids
}

// FindContainerIP returns the ip of the specified container
func (d *Daemon) FindContainerIP(t testingT, id string) string {
	cli := d.NewClientT(t)
	defer cli.Close()

	i, err := cli.ContainerInspect(context.Background(), id)
	assert.NilError(t, err)
	return i.NetworkSettings.IPAddress
}
