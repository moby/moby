// +build linux, experimental

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

var pluginName = "tiborvass/no-remove"

// TestDaemonRestartWithPluginEnabled tests state restore for an enabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginEnabled(c *check.C) {
	testRequires(c, Network)
	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pluginName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if out, err := s.d.Cmd("plugin", "disable", pluginName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pluginName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	out, err := s.d.Cmd("plugin", "ls")
	if err != nil {
		c.Fatalf("Could not list plugins: %v %s", err, out)
	}
	c.Assert(out, checker.Contains, pluginName)
	c.Assert(out, checker.Contains, "true")
}

// TestDaemonRestartWithPluginDisabled tests state restore for a disabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginDisabled(c *check.C) {
	testRequires(c, Network)
	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pluginName, "--disable"); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if out, err := s.d.Cmd("plugin", "remove", pluginName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	out, err := s.d.Cmd("plugin", "ls")
	if err != nil {
		c.Fatalf("Could not list plugins: %v %s", err, out)
	}
	c.Assert(out, checker.Contains, pluginName)
	c.Assert(out, checker.Contains, "false")
}

// TestDaemonKillLiveRestoreWithPlugins SIGKILLs daemon started with --live-restore.
// Plugins should continue to run.
func (s *DockerDaemonSuite) TestDaemonKillLiveRestoreWithPlugins(c *check.C) {
	testRequires(c, Network)
	if err := s.d.Start("--live-restore"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pluginName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		if err := s.d.Restart("--live-restore"); err != nil {
			c.Fatalf("Could not restart daemon: %v", err)
		}
		if out, err := s.d.Cmd("plugin", "disable", pluginName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pluginName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.Kill(); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	cmd := exec.Command("pgrep", "-f", "plugin-no-remove")
	if out, ec, err := runCommandWithOutput(cmd); ec != 0 {
		c.Fatalf("Expected exit code '0', got %d err: %v output: %s ", ec, err, out)
	}
}

// TestDaemonShutdownLiveRestoreWithPlugins SIGTERMs daemon started with --live-restore.
// Plugins should continue to run.
func (s *DockerDaemonSuite) TestDaemonShutdownLiveRestoreWithPlugins(c *check.C) {
	testRequires(c, Network)
	if err := s.d.Start("--live-restore"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pluginName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		if err := s.d.Restart("--live-restore"); err != nil {
			c.Fatalf("Could not restart daemon: %v", err)
		}
		if out, err := s.d.Cmd("plugin", "disable", pluginName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pluginName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.cmd.Process.Signal(os.Interrupt); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	cmd := exec.Command("pgrep", "-f", "plugin-no-remove")
	if out, ec, err := runCommandWithOutput(cmd); ec != 0 {
		c.Fatalf("Expected exit code '0', got %d err: %v output: %s ", ec, err, out)
	}
}

// TestDaemonShutdownWithPlugins shuts down running plugins.
func (s *DockerDaemonSuite) TestDaemonShutdownWithPlugins(c *check.C) {
	testRequires(c, Network)
	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pluginName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if err := s.d.Restart(); err != nil {
			c.Fatalf("Could not restart daemon: %v", err)
		}
		if out, err := s.d.Cmd("plugin", "disable", pluginName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pluginName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.cmd.Process.Signal(os.Interrupt); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	for {
		if err := syscall.Kill(s.d.cmd.Process.Pid, 0); err == syscall.ESRCH {
			break
		}
	}

	cmd := exec.Command("pgrep", "-f", "plugin-no-remove")
	if out, ec, err := runCommandWithOutput(cmd); ec != 1 {
		c.Fatalf("Expected exit code '1', got %d err: %v output: %s ", ec, err, out)
	}
}

// TestVolumePlugin tests volume creation using a plugin.
func (s *DockerDaemonSuite) TestVolumePlugin(c *check.C) {
	testRequires(c, Network)
	volName := "plugin-volume"
	volRoot := "/data"
	destDir := "/tmp/data/"
	destFile := "foo"

	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	out, err := s.d.Cmd("plugin", "install", pluginName, "--grant-all-permissions")
	if err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		if out, err := s.d.Cmd("plugin", "disable", pluginName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pluginName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	out, err = s.d.Cmd("volume", "create", "-d", pluginName, volName)
	if err != nil {
		c.Fatalf("Could not create volume: %v %s", err, out)
	}
	defer func() {
		if out, err := s.d.Cmd("volume", "remove", volName); err != nil {
			c.Fatalf("Could not remove volume: %v %s", err, out)
		}
	}()

	out, err = s.d.Cmd("volume", "ls")
	if err != nil {
		c.Fatalf("Could not list volume: %v %s", err, out)
	}
	c.Assert(out, checker.Contains, volName)
	c.Assert(out, checker.Contains, pluginName)

	mountPoint, err := s.d.Cmd("volume", "inspect", volName, "--format", "{{.Mountpoint}}")
	if err != nil {
		c.Fatalf("Could not inspect volume: %v %s", err, mountPoint)
	}
	mountPoint = strings.TrimSpace(mountPoint)

	out, err = s.d.Cmd("run", "--rm", "-v", volName+":"+destDir, "busybox", "touch", destDir+destFile)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	path := filepath.Join(mountPoint, destFile)
	_, err = os.Lstat(path)
	c.Assert(err, checker.IsNil)

	// tiborvass/no-remove is a volume plugin that persists data on disk at /data,
	// even after the volume is removed. So perform an explicit filesystem cleanup.
	os.RemoveAll(volRoot)
}
