package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"gotest.tools/v3/assert"
)

// ActiveContainers returns the list of ids of the currently running containers
func (d *Daemon) ActiveContainers(t testing.TB) []string {
	t.Helper()
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
func (d *Daemon) FindContainerIP(t testing.TB, id string) string {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	i, err := cli.ContainerInspect(context.Background(), id)
	assert.NilError(t, err)
	return i.NetworkSettings.IPAddress
}

func (d *Daemon) ContainerExitCode(t testing.TB, cid string) (int64, error) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	resp, err := cli.ContainerWait(context.Background(), cid, container.WaitConditionNotRunning)
	select {
	case err := <-err:
		return 0, err
	case resp := <-resp:
		if resp.Error != nil {
			return 0, errors.New(resp.Error.Message)
		}
		return resp.StatusCode, nil
	}
}
