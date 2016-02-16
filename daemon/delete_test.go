package daemon

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/container"
	"github.com/docker/docker/container/state"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
)

func TestContainerDoubleDelete(t *testing.T) {
	tmp, err := ioutil.TempDir("", "docker-daemon-unix-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	daemon := &Daemon{
		repository: tmp,
		root:       tmp,
	}
	daemon.containers = container.NewMemoryStore()

	container := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:     "test",
			State:  state.NewState(),
			Config: &containertypes.Config{},
		},
	}
	daemon.containers.Add(container.ID, container)

	// Mark the container as having a delete in progress
	container.SetRemovalInProgressLocking()

	// Try to remove the container when it's start is removalInProgress.
	// It should ignore the container and not return an error.
	if err := daemon.ContainerRm(container.ID, &types.ContainerRmConfig{ForceRemove: true}); err != nil {
		t.Fatal(err)
	}
}
