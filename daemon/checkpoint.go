package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/api/types/checkpoint"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/names"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/errdefs"
)

func validateCheckpointID(checkpointID string) error {
	if !names.RestrictedNamePattern.MatchString(checkpointID) || strings.HasSuffix(checkpointID, ".") {
		return errdefs.InvalidParameter(fmt.Errorf("invalid checkpoint ID %q, only %s are allowed", checkpointID, names.RestrictedNameChars))
	}
	return nil
}

func checkpointRoot(checkDir, ctrCheckpointDir string) string {
	if checkDir != "" {
		return checkDir
	}
	return ctrCheckpointDir
}

// getCheckpointDir resolves and verifies a named checkpoint directory.
func getCheckpointDir(checkDir, checkpointID, ctrName, ctrID, ctrCheckpointDir string, create bool) (string, error) {
	checkpointDir := checkpointRoot(checkDir, ctrCheckpointDir)
	var err2 error
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

// getCheckpointRoot resolves the checkpoint root used by CheckpointList.
func getCheckpointRoot(checkDir, ctrName, ctrCheckpointDir string) (string, error) {
	checkpointDir := checkpointRoot(checkDir, ctrCheckpointDir)
	// The API intentionally supports caller-selected checkpoint roots.
	stat, err := os.Stat(checkpointDir)
	if err != nil {
		if checkDir != "" && errors.Is(err, os.ErrNotExist) {
			return checkpointDir, errdefs.NotFound(fmt.Errorf("checkpoint directory %q does not exist for container %s: %w", checkpointDir, ctrName, err))
		}
		return checkpointDir, fmt.Errorf("failed to stat checkpoint directory %q for container %s: %w", checkpointDir, ctrName, err)
	}
	if !stat.IsDir() {
		if checkDir != "" {
			return checkpointDir, errdefs.InvalidParameter(fmt.Errorf("checkpoint directory %q exists but is not a directory", checkpointDir))
		}
		return checkpointDir, fmt.Errorf("%s exists and is not a directory", checkpointDir)
	}
	return checkpointDir, nil
}

// CheckpointCreate checkpoints the process running in a container with CRIU
func (daemon *Daemon) CheckpointCreate(name string, config checkpoint.CreateRequest) error {
	if err := validateCheckpointID(config.CheckpointID); err != nil {
		return err
	}

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

	checkpointDir, err := getCheckpointDir(config.CheckpointDir, config.CheckpointID, name, container.ID, container.CheckpointDir(), true)
	if err != nil {
		return fmt.Errorf("cannot checkpoint container %s: %w", name, err)
	}

	err = tsk.CreateCheckpoint(context.Background(), checkpointDir, config.Exit)
	if err != nil {
		os.RemoveAll(checkpointDir)
		return fmt.Errorf("Cannot checkpoint container %s: %s", name, err)
	}

	daemon.LogContainerEvent(container, events.ActionCheckpoint)

	return nil
}

// CheckpointDelete deletes the specified checkpoint
func (daemon *Daemon) CheckpointDelete(name string, config backend.CheckpointDeleteOptions) error {
	if err := validateCheckpointID(config.CheckpointID); err != nil {
		return err
	}

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
func (daemon *Daemon) CheckpointList(name string, config backend.CheckpointListOptions) ([]checkpoint.Summary, error) {
	var out []checkpoint.Summary

	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	checkpointDir, err := getCheckpointRoot(config.CheckpointDir, name, container.CheckpointDir())
	if err != nil {
		if config.CheckpointDir == "" && errors.Is(err, os.ErrNotExist) {
			return out, nil
		}
		return nil, err
	}

	dirs, err := os.ReadDir(checkpointDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if config.CheckpointDir == "" {
				return out, nil
			}
			return nil, errdefs.NotFound(fmt.Errorf("checkpoint directory %q does not exist for container %s: %w", checkpointDir, name, err))
		}
		return nil, err
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		cpt := checkpoint.Summary{Name: d.Name()}
		out = append(out, cpt)
	}

	return out, nil
}
