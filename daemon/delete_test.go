package daemon

import (
	"io/ioutil"
	"os"

	"github.com/docker/docker/container"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestContainerDoubleDelete(c *check.C) {
	tmp, err := ioutil.TempDir("", "docker-daemon-unix-test-")
	if err != nil {
		c.Fatal(err)
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

	// Try to remove the container when its start is removalInProgress.
	// It should ignore the container and not return an error.
	if err := daemon.ContainerRm(container.ID, &types.ContainerRmConfig{ForceRemove: true}); err != nil {
		c.Fatal(err)
	}
}
