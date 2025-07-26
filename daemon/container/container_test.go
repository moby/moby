package container

import (
	"fmt"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/daemon/logger/jsonfilelog"
	"gotest.tools/v3/assert"
)

func TestContainerStopSignal(t *testing.T) {
	c := &Container{
		Config: &container.Config{},
	}

	s := c.StopSignal()
	assert.Equal(t, s, defaultStopSignal)

	c = &Container{
		Config: &container.Config{StopSignal: "SIGKILL"},
	}
	s = c.StopSignal()
	expected := syscall.SIGKILL
	assert.Equal(t, s, expected)

	c = &Container{
		Config: &container.Config{StopSignal: "NOSUCHSIGNAL"},
	}
	s = c.StopSignal()
	assert.Equal(t, s, defaultStopSignal)
}

func TestContainerStopTimeout(t *testing.T) {
	c := &Container{
		Config: &container.Config{},
	}

	s := c.StopTimeout()
	assert.Equal(t, s, defaultStopTimeout)

	stopTimeout := 15
	c = &Container{
		Config: &container.Config{StopTimeout: &stopTimeout},
	}
	s = c.StopTimeout()
	assert.Equal(t, s, stopTimeout)
}

func TestContainerSecretReferenceDestTarget(t *testing.T) {
	ref := &swarm.SecretReference{
		File: &swarm.SecretReferenceFileTarget{
			Name: "app",
		},
	}

	d := getSecretTargetPath(ref)
	expected := filepath.Join(containerSecretMountPath, "app")
	assert.Equal(t, d, expected)
}

func TestContainerLogPathSetForJSONFileLogger(t *testing.T) {
	containerRoot := t.TempDir()

	c := &Container{
		Config: &container.Config{},
		HostConfig: &container.HostConfig{
			LogConfig: container.LogConfig{
				Type: jsonfilelog.Name,
			},
		},
		ID:   t.Name(),
		Root: containerRoot,
	}

	logger, err := c.StartLogger()
	assert.NilError(t, err)
	defer func() {
		assert.NilError(t, logger.Close())
	}()

	expectedLogPath, err := filepath.Abs(filepath.Join(containerRoot, fmt.Sprintf("%s-json.log", c.ID)))
	assert.NilError(t, err)
	assert.Equal(t, c.LogPath, expectedLogPath)
}

func TestContainerLogPathSetForRingLogger(t *testing.T) {
	containerRoot := t.TempDir()

	c := &Container{
		Config: &container.Config{},
		HostConfig: &container.HostConfig{
			LogConfig: container.LogConfig{
				Type: jsonfilelog.Name,
				Config: map[string]string{
					"mode": string(container.LogModeNonBlock),
				},
			},
		},
		ID:   t.Name(),
		Root: containerRoot,
	}

	logger, err := c.StartLogger()
	assert.NilError(t, err)
	defer func() {
		assert.NilError(t, logger.Close())
	}()

	expectedLogPath, err := filepath.Abs(filepath.Join(containerRoot, fmt.Sprintf("%s-json.log", c.ID)))
	assert.NilError(t, err)
	assert.Equal(t, c.LogPath, expectedLogPath)
}
