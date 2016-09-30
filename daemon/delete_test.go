package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
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
			State:  container.NewState(),
			Config: &containertypes.Config{},
		},
	}
	daemon.containers.Add(container.ID, container)

	// Mark the container as having a delete in progress
	container.SetRemovalInProgress()

	// Try to remove the container when its state is removalInProgress.
	// It should return an error indicating it is under removal progress.
	if err := daemon.ContainerRm(container.ID, &types.ContainerRmConfig{ForceRemove: true}); err == nil {
		t.Fatalf("expected err: %v, got nil", fmt.Sprintf("removal of container %s is already in progress", container.ID))
	}
}
