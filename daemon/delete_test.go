package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func newDaemonWithTmpRoot(t *testing.T) (*Daemon, func()) {
	tmp, err := os.MkdirTemp("", "docker-daemon-unix-test-")
	assert.NilError(t, err)
	d := &Daemon{
		repository: tmp,
		root:       tmp,
	}
	d.containers = container.NewMemoryStore()
	return d, func() { os.RemoveAll(tmp) }
}

func newContainerWithState(state *container.State) *container.Container {
	return &container.Container{
		ID:     "test",
		State:  state,
		Config: &containertypes.Config{},
	}
}

// TestContainerDelete tests that a useful error message and instructions is
// given when attempting to remove a container (#30842)
func TestContainerDelete(t *testing.T) {
	tests := []struct {
		doc           string
		errMsg        string
		initContainer func() *container.Container
	}{
		{
			doc:    "paused container",
			errMsg: "container is paused and must be unpaused first",
			initContainer: func() *container.Container {
				return newContainerWithState(&container.State{Paused: true, Running: true})
			},
		},
		{
			doc:    "restarting container",
			errMsg: "container is restarting: stop the container before removing or force remove",
			initContainer: func() *container.Container {
				c := newContainerWithState(container.NewState())
				c.SetRunning(nil, nil, time.Now())
				c.SetRestarting(&container.ExitStatus{})
				return c
			},
		},
		{
			doc:    "running container",
			errMsg: "container is running: stop the container before removing or force remove",
			initContainer: func() *container.Container {
				return newContainerWithState(&container.State{Running: true})
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			c := tc.initContainer()
			d, cleanup := newDaemonWithTmpRoot(t)
			defer cleanup()
			d.containers.Add(c.ID, c)

			err := d.ContainerRm(c.ID, &backend.ContainerRmConfig{ForceRemove: false})
			assert.Check(t, is.ErrorType(err, errdefs.IsConflict))
			assert.Check(t, is.ErrorContains(err, tc.errMsg))
		})
	}
}

func TestContainerDoubleDelete(t *testing.T) {
	c := newContainerWithState(container.NewState())

	// Mark the container as having a delete in progress
	c.SetRemovalInProgress()

	d, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()
	d.containers.Add(c.ID, c)

	// Try to remove the container when its state is removalInProgress.
	// It should return an error indicating it is under removal progress.
	err := d.ContainerRm(c.ID, &backend.ContainerRmConfig{ForceRemove: true})
	assert.Check(t, is.ErrorContains(err, fmt.Sprintf("removal of container %s is already in progress", c.ID)))
}
