package container

import (
	"fmt"
	"os"
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

func TestSetupWorkingDirectory(t *testing.T) {
	uid, gid := os.Getuid(), os.Getgid()

	t.Run("creates nested workdir under the container root", func(t *testing.T) {
		base := t.TempDir()
		c := &Container{BaseFS: base, Config: &container.Config{WorkingDir: "/a/b/c"}}
		assert.NilError(t, c.SetupWorkingDirectory(uid, gid))
		fi, err := os.Stat(filepath.Join(base, "a", "b", "c"))
		assert.NilError(t, err)
		assert.Assert(t, fi.IsDir())
	})

	t.Run("empty workdir is a no-op", func(t *testing.T) {
		c := &Container{BaseFS: t.TempDir(), Config: &container.Config{}}
		assert.NilError(t, c.SetupWorkingDirectory(uid, gid))
	})

	t.Run("workdir over an existing file fails", func(t *testing.T) {
		base := t.TempDir()
		assert.NilError(t, os.WriteFile(filepath.Join(base, "file"), nil, 0o644))
		c := &Container{BaseFS: base, Config: &container.Config{WorkingDir: "/file"}}
		assert.ErrorContains(t, c.SetupWorkingDirectory(uid, gid), "not a directory")
	})

	t.Run("symlinked component cannot escape the container root", func(t *testing.T) {
		base, outside := t.TempDir(), t.TempDir()
		if err := os.Symlink(outside, filepath.Join(base, "link")); err != nil {
			t.Skipf("symlinks unsupported: %v", err)
		}
		c := &Container{BaseFS: base, Config: &container.Config{WorkingDir: "/link/evil"}}
		err := c.SetupWorkingDirectory(uid, gid)
		_, statErr := os.Stat(filepath.Join(outside, "evil"))
		assert.Assert(t, os.IsNotExist(statErr), "creation escaped the container root (err=%v)", err)
	})
}

func TestScopedMkdirAllAndChown(t *testing.T) {
	uid, gid := os.Getuid(), os.Getgid()

	t.Run("creates missing parents", func(t *testing.T) {
		base := t.TempDir()
		root, err := os.OpenRoot(base)
		assert.NilError(t, err)
		defer root.Close()
		assert.NilError(t, scopedMkdirAllAndChown(root, "a/b/c", 0o755, uid, gid))
		fi, err := os.Stat(filepath.Join(base, "a", "b", "c"))
		assert.NilError(t, err)
		assert.Assert(t, fi.IsDir())
	})

	t.Run("tolerates existing parents", func(t *testing.T) {
		base := t.TempDir()
		assert.NilError(t, os.Mkdir(filepath.Join(base, "a"), 0o755))
		root, err := os.OpenRoot(base)
		assert.NilError(t, err)
		defer root.Close()
		assert.NilError(t, scopedMkdirAllAndChown(root, "a/b", 0o755, uid, gid))
		fi, err := os.Stat(filepath.Join(base, "a", "b"))
		assert.NilError(t, err)
		assert.Assert(t, fi.IsDir())
	})

	t.Run("errors when a component is not a directory", func(t *testing.T) {
		base := t.TempDir()
		assert.NilError(t, os.WriteFile(filepath.Join(base, "f"), nil, 0o644))
		root, err := os.OpenRoot(base)
		assert.NilError(t, err)
		defer root.Close()
		assert.ErrorIs(t, scopedMkdirAllAndChown(root, "f/x", 0o755, uid, gid), syscall.ENOTDIR)
	})

	t.Run("does not follow a symlink out of root", func(t *testing.T) {
		base, outside := t.TempDir(), t.TempDir()
		if err := os.Symlink(outside, filepath.Join(base, "link")); err != nil {
			t.Skipf("symlinks unsupported: %v", err)
		}
		root, err := os.OpenRoot(base)
		assert.NilError(t, err)
		defer root.Close()
		_ = scopedMkdirAllAndChown(root, "link/evil", 0o755, uid, gid)
		_, statErr := os.Stat(filepath.Join(outside, "evil"))
		assert.Assert(t, os.IsNotExist(statErr), "creation escaped the container root")
	})
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
