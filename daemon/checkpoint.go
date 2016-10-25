package daemon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
)

// CheckpointCreate checkpoints the process running in a container with CRIU
func (daemon *Daemon) CheckpointCreate(name string, config types.CheckpointCreateOptions) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	if !container.IsRunning() {
		return fmt.Errorf("Container %s not running", name)
	}

	err = daemon.containerd.CreateCheckpoint(container.ID, config.CheckpointID, container.CheckpointDir(), config.Exit)
	if err != nil {
		return fmt.Errorf("Cannot checkpoint container %s: %s", name, err)
	}

	daemon.LogContainerEvent(container, "checkpoint")

	return nil
}

// CheckpointDelete deletes the specified checkpoint
func (daemon *Daemon) CheckpointDelete(name string, checkpoint string) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	checkpointDir := container.CheckpointDir()
	return os.RemoveAll(filepath.Join(checkpointDir, checkpoint))
}

// CheckpointList lists all checkpoints of the specified container
func (daemon *Daemon) CheckpointList(name string) ([]types.Checkpoint, error) {
	var out []types.Checkpoint

	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	checkpointDir := container.CheckpointDir()
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return nil, err
	}

	dirs, err := ioutil.ReadDir(checkpointDir)
	if err != nil {
		return nil, err
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		path := filepath.Join(checkpointDir, d.Name(), "config.json")
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var cpt types.Checkpoint
		if err := json.Unmarshal(data, &cpt); err != nil {
			return nil, err
		}
		out = append(out, cpt)
	}

	return out, nil
}
