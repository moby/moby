package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/names"
)

var (
	validCheckpointNameChars   = names.RestrictedNameChars
	validCheckpointNamePattern = names.RestrictedNamePattern
)

// getCheckpointDir verifies checkpoint directory for create,remove, list options and checks if checkpoint already exists
func getCheckpointDir(checkDir, checkpointID, ctrName, ctrID, ctrCheckpointDir string, create bool) (string, error) {
	var checkpointDir string
	var err2 error
	if checkDir != "" {
		checkpointDir = checkDir
	} else {
		checkpointDir = ctrCheckpointDir
	}
	checkpointAbsDir := filepath.Join(checkpointDir, checkpointID)
	stat, err := os.Stat(checkpointAbsDir)
	if create {
		switch {
		case err == nil && stat.IsDir():
			err2 = fmt.Errorf("checkpoint with name %s already exists for container %s", checkpointID, ctrName)
		case err != nil && os.IsNotExist(err):
			err2 = os.MkdirAll(checkpointAbsDir, 0o700)
		case err != nil:
			err2 = err
		default:
			err2 = fmt.Errorf("%s exists and is not a directory", checkpointAbsDir)
		}
	} else {
		switch {
		case err != nil:
			err2 = fmt.Errorf("checkpoint %s does not exist for container %s", checkpointID, ctrName)
		case stat.IsDir():
			err2 = nil
		default:
			err2 = fmt.Errorf("%s exists and is not a directory", checkpointAbsDir)
		}
	}
	return checkpointAbsDir, err2
}

// CheckpointCreate checkpoints the process running in a container with CRIU
func (daemon *Daemon) CheckpointCreate(name string, config types.CheckpointCreateOptions) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	container.Lock()
	tsk, err := container.GetRunningTask()
	container.Unlock()
	if err != nil {
		return err
	}

	if !validCheckpointNamePattern.MatchString(config.CheckpointID) {
		return fmt.Errorf("Invalid checkpoint ID (%s), only %s are allowed", config.CheckpointID, validCheckpointNameChars)
	}

	checkpointDir, err := getCheckpointDir(config.CheckpointDir, config.CheckpointID, name, container.ID, container.CheckpointDir(), true)
	if err != nil {
		return fmt.Errorf("cannot checkpoint container %s: %s", name, err)
	}

	err = tsk.CreateCheckpoint(context.Background(), checkpointDir, config.Exit)
	if err != nil {
		os.RemoveAll(checkpointDir)
		return fmt.Errorf("Cannot checkpoint container %s: %s", name, err)
	}

	daemon.LogContainerEvent(container, "checkpoint")

	return nil
}

// CheckpointDelete deletes the specified checkpoint
func (daemon *Daemon) CheckpointDelete(name string, config types.CheckpointDeleteOptions) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	checkpointDir, err := getCheckpointDir(config.CheckpointDir, config.CheckpointID, name, container.ID, container.CheckpointDir(), false)
	if err == nil {
		return os.RemoveAll(checkpointDir)
	}
	return err
}

// CheckpointList lists all checkpoints of the specified container
func (daemon *Daemon) CheckpointList(name string, config types.CheckpointListOptions) ([]types.Checkpoint, error) {
	var out []types.Checkpoint

	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	checkpointDir, err := getCheckpointDir(config.CheckpointDir, "", name, container.ID, container.CheckpointDir(), false)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return nil, err
	}

	dirs, err := os.ReadDir(checkpointDir)
	if err != nil {
		return nil, err
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		cpt := types.Checkpoint{Name: d.Name()}
		out = append(out, cpt)
	}

	return out, nil
}
