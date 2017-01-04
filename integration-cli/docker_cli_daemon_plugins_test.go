// +build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/mount"
	"github.com/go-check/check"
)

// TestDaemonRestartWithPluginEnabled tests state restore for an enabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginEnabled(c *check.C) {
	testRequires(c, IsAmd64, Network)

	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
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
	c.Assert(out, checker.Contains, pName)
	c.Assert(out, checker.Contains, "true")
}

// TestDaemonRestartWithPluginDisabled tests state restore for a disabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginDisabled(c *check.C) {
	testRequires(c, IsAmd64, Network)

	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName, "--disable"); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
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
	c.Assert(out, checker.Contains, pName)
	c.Assert(out, checker.Contains, "false")
}

// TestDaemonKillLiveRestoreWithPlugins SIGKILLs daemon started with --live-restore.
// Plugins should continue to run.
func (s *DockerDaemonSuite) TestDaemonKillLiveRestoreWithPlugins(c *check.C) {
	testRequires(c, IsAmd64, Network)

	if err := s.d.Start("--live-restore"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		if err := s.d.Restart("--live-restore"); err != nil {
			c.Fatalf("Could not restart daemon: %v", err)
		}
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.Kill(); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	cmd := exec.Command("pgrep", "-f", pluginProcessName)
	if out, ec, err := runCommandWithOutput(cmd); ec != 0 {
		c.Fatalf("Expected exit code '0', got %d err: %v output: %s ", ec, err, out)
	}
}

// TestDaemonShutdownLiveRestoreWithPlugins SIGTERMs daemon started with --live-restore.
// Plugins should continue to run.
func (s *DockerDaemonSuite) TestDaemonShutdownLiveRestoreWithPlugins(c *check.C) {
	testRequires(c, IsAmd64, Network)

	if err := s.d.Start("--live-restore"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		if err := s.d.Restart("--live-restore"); err != nil {
			c.Fatalf("Could not restart daemon: %v", err)
		}
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.cmd.Process.Signal(os.Interrupt); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	cmd := exec.Command("pgrep", "-f", pluginProcessName)
	if out, ec, err := runCommandWithOutput(cmd); ec != 0 {
		c.Fatalf("Expected exit code '0', got %d err: %v output: %s ", ec, err, out)
	}
}

// TestDaemonShutdownWithPlugins shuts down running plugins.
func (s *DockerDaemonSuite) TestDaemonShutdownWithPlugins(c *check.C) {
	testRequires(c, IsAmd64, Network, SameHostDaemon)

	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if err := s.d.Restart(); err != nil {
			c.Fatalf("Could not restart daemon: %v", err)
		}
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
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

	cmd := exec.Command("pgrep", "-f", pluginProcessName)
	if out, ec, err := runCommandWithOutput(cmd); ec != 1 {
		c.Fatalf("Expected exit code '1', got %d err: %v output: %s ", ec, err, out)
	}

	s.d.Start("--live-restore")
	cmd = exec.Command("pgrep", "-f", pluginProcessName)
	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

// TestVolumePlugin tests volume creation using a plugin.
func (s *DockerDaemonSuite) TestVolumePlugin(c *check.C) {
	testRequires(c, IsAmd64, Network)

	volName := "plugin-volume"
	destDir := "/tmp/data/"
	destFile := "foo"

	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	out, err := s.d.Cmd("plugin", "install", pName, "--grant-all-permissions")
	if err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	pluginID, err := s.d.Cmd("plugin", "inspect", "-f", "{{.Id}}", pName)
	pluginID = strings.TrimSpace(pluginID)
	if err != nil {
		c.Fatalf("Could not retrieve plugin ID: %v %s", err, pluginID)
	}
	mountpointPrefix := filepath.Join(s.d.RootDir(), "plugins", pluginID, "rootfs")
	defer func() {
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}

		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}

		exists, err := existsMountpointWithPrefix(mountpointPrefix)
		c.Assert(err, checker.IsNil)
		c.Assert(exists, checker.Equals, false)

	}()

	out, err = s.d.Cmd("volume", "create", "-d", pName, volName)
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
	c.Assert(out, checker.Contains, pName)

	mountPoint, err := s.d.Cmd("volume", "inspect", volName, "--format", "{{.Mountpoint}}")
	if err != nil {
		c.Fatalf("Could not inspect volume: %v %s", err, mountPoint)
	}
	mountPoint = strings.TrimSpace(mountPoint)

	out, err = s.d.Cmd("run", "--rm", "-v", volName+":"+destDir, "busybox", "touch", destDir+destFile)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	path := filepath.Join(s.d.RootDir(), "plugins", pluginID, "rootfs", mountPoint, destFile)
	_, err = os.Lstat(path)
	c.Assert(err, checker.IsNil)

	exists, err := existsMountpointWithPrefix(mountpointPrefix)
	c.Assert(err, checker.IsNil)
	c.Assert(exists, checker.Equals, true)
}

func (s *DockerDaemonSuite) TestGraphdriverPlugin(c *check.C) {
	testRequires(c, Network, IsAmd64, DaemonIsLinux, overlay2Supported, ExperimentalDaemon)

	s.d.Start()

	// install the plugin
	plugin := "cpuguy83/docker-overlay2-graphdriver-plugin"
	out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", plugin)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// restart the daemon with the plugin set as the storage driver
	s.d.Restart("-s", plugin, "--storage-opt", "overlay2.override_kernel_check=1")

	// run a container
	out, err = s.d.Cmd("run", "--rm", "busybox", "true") // this will pull busybox using the plugin
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestPluginVolumeRemoveOnRestart(c *check.C) {
	testRequires(c, DaemonIsLinux, Network, IsAmd64)

	s.d.Start("--live-restore=true")

	out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Contains, pName)

	out, err = s.d.Cmd("volume", "create", "--driver", pName, "test")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	s.d.Restart("--live-restore=true")

	out, err = s.d.Cmd("plugin", "disable", pName)
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "in use")

	out, err = s.d.Cmd("volume", "rm", "test")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("plugin", "disable", pName)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("plugin", "rm", pName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func existsMountpointWithPrefix(mountpointPrefix string) (bool, error) {
	mounts, err := mount.GetMounts()
	if err != nil {
		return false, err
	}
	for _, mnt := range mounts {
		if strings.HasPrefix(mnt.Mountpoint, mountpointPrefix) {
			return true, nil
		}
	}
	return false, nil
}
