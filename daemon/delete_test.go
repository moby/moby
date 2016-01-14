package daemon

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/container"
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
	daemon.containers = &contStore{s: make(map[string]*container.Container)}

	container := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:     "test",
			State:  container.NewState(),
			Config: &containertypes.Config{},
		},
	}
	daemon.containers.Add(container.ID, container)

	// Mark the container as having a delete in progress
	if err := container.SetRemovalInProgress(); err != nil {
		t.Fatal(err)
	}

	// Try to remove the container when it's start is removalInProgress.
	// It should ignore the container and not return an error.
	if err := daemon.ContainerRm(container.ID, &types.ContainerRmConfig{ForceRemove: true}); err != nil {
		t.Fatal(err)
	}
}
