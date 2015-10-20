package daemon

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/runconfig"
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

	container := &Container{
		CommonContainer: CommonContainer{
			State:  NewState(),
			Config: &runconfig.Config{},
		},
	}

	// Mark the container as having a delete in progress
	if err := container.setRemovalInProgress(); err != nil {
		t.Fatal(err)
	}

	// Try to remove the container when it's start is removalInProgress.
	// It should ignore the container and not return an error.
	if err := daemon.rm(container, true); err != nil {
		t.Fatal(err)
	}
}
