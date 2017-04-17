package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func newDaemonWithTmpRoot(t *testing.T) (*Daemon, func()) {
	tmp, err := ioutil.TempDir("", "docker-daemon-unix-test-")
	require.NoError(t, err)
	d := &Daemon{
		repository: tmp,
		root:       tmp,
	}
	d.containers = container.NewMemoryStore()
	return d, func() { os.RemoveAll(tmp) }
}

// TestContainerDeletePaused tests that a useful error message and instructions is given when attempting
// to remove a paused container (#30842)
func TestContainerDeletePaused(t *testing.T) {
	c := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:     "test",
			State:  &container.State{Paused: true, Running: true},
			Config: &containertypes.Config{},
		},
	}

	d, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()
	d.containers.Add(c.ID, c)

	err := d.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: false})

	testutil.ErrorContains(t, err, "cannot remove a paused container")
	testutil.ErrorContains(t, err, "Unpause and then stop the container before attempting removal or force remove")
}

// TestContainerDeleteRestarting tests that a useful error message and instructions is given when attempting
// to remove a container that is restarting (#30842)
func TestContainerDeleteRestarting(t *testing.T) {
	c := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:     "test",
			State:  container.NewState(),
			Config: &containertypes.Config{},
		},
	}

	c.SetRunning(0, true)
	c.SetRestarting(&container.ExitStatus{})

	d, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()
	d.containers.Add(c.ID, c)

	err := d.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: false})
	testutil.ErrorContains(t, err, "cannot remove a restarting container")
	testutil.ErrorContains(t, err, "Stop the container before attempting removal or force remove")
}

// TestContainerDeleteRunning tests that a useful error message and instructions is given when attempting
// to remove a running container (#30842)
func TestContainerDeleteRunning(t *testing.T) {
	c := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:     "test",
			State:  &container.State{Running: true},
			Config: &containertypes.Config{},
		},
	}

	d, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()
	d.containers.Add(c.ID, c)

	err := d.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: false})
	testutil.ErrorContains(t, err, "cannot remove a running container")
}

func TestContainerDoubleDelete(t *testing.T) {
	c := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:     "test",
			State:  container.NewState(),
			Config: &containertypes.Config{},
		},
	}

	// Mark the container as having a delete in progress
	c.SetRemovalInProgress()

	d, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()
	d.containers.Add(c.ID, c)

	// Try to remove the container when its state is removalInProgress.
	// It should return an error indicating it is under removal progress.
	err := d.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: true})
	testutil.ErrorContains(t, err, fmt.Sprintf("removal of container %s is already in progress", c.ID))
}
