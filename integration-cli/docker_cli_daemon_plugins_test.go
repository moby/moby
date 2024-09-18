//go:build linux

package main

import (
	"strings"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

// TestDaemonRestartWithPluginEnabled tests state restore for an enabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginEnabled(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c)

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

	s.d.Restart(c)

	out, err := s.d.Cmd("plugin", "ls")
	if err != nil {
		c.Fatalf("Could not list plugins: %v %s", err, out)
	}
	assert.Assert(c, strings.Contains(out, pName))
	assert.Assert(c, strings.Contains(out, "true"))
}

// TestDaemonRestartWithPluginDisabled tests state restore for a disabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginDisabled(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c)

	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName, "--disable"); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	s.d.Restart(c)

	out, err := s.d.Cmd("plugin", "ls")
	if err != nil {
		c.Fatalf("Could not list plugins: %v %s", err, out)
	}
	assert.Assert(c, strings.Contains(out, pName))
	assert.Assert(c, strings.Contains(out, "false"))
}

// TestDaemonKillLiveRestoreWithPlugins SIGKILLs daemon started with --live-restore.
// Plugins should continue to run.
func (s *DockerDaemonSuite) TestDaemonKillLiveRestoreWithPlugins(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c, "--live-restore")
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		s.d.Restart(c, "--live-restore")
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

	icmd.RunCommand("pgrep", "-f", pluginProcessName).Assert(c, icmd.Success)
}

// TestDaemonShutdownLiveRestoreWithPlugins SIGTERMs daemon started with --live-restore.
// Plugins should continue to run.
func (s *DockerDaemonSuite) TestDaemonShutdownLiveRestoreWithPlugins(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c, "--live-restore")
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}
	defer func() {
		s.d.Restart(c, "--live-restore")
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.Interrupt(); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	icmd.RunCommand("pgrep", "-f", pluginProcessName).Assert(c, icmd.Success)
}

// TestDaemonShutdownWithPlugins shuts down running plugins.
func (s *DockerDaemonSuite) TestDaemonShutdownWithPlugins(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c)
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		s.d.Restart(c)
		if out, err := s.d.Cmd("plugin", "disable", pName); err != nil {
			c.Fatalf("Could not disable plugin: %v %s", err, out)
		}
		if out, err := s.d.Cmd("plugin", "remove", pName); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	if err := s.d.Interrupt(); err != nil {
		c.Fatalf("Could not kill daemon: %v", err)
	}

	for {
		if err := unix.Kill(s.d.Pid(), 0); err == unix.ESRCH {
			break
		}
	}

	icmd.RunCommand("pgrep", "-f", pluginProcessName).Assert(c, icmd.Expected{
		ExitCode: 1,
		Error:    "exit status 1",
	})

	s.d.Start(c)
	icmd.RunCommand("pgrep", "-f", pluginProcessName).Assert(c, icmd.Success)
}

// TestDaemonKillWithPlugins leaves plugins running.
func (s *DockerDaemonSuite) TestDaemonKillWithPlugins(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c)
	if out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName); err != nil {
		c.Fatalf("Could not install plugin: %v %s", err, out)
	}

	defer func() {
		s.d.Restart(c)
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

	// assert that plugins are running.
	icmd.RunCommand("pgrep", "-f", pluginProcessName).Assert(c, icmd.Success)
}

// TestVolumePlugin tests volume creation using a plugin.
func (s *DockerDaemonSuite) TestVolumePlugin(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	volName := "plugin-volume"
	destDir := "/tmp/data/"
	destFile := "foo"

	s.d.Start(c)
	out, err := s.d.Cmd("plugin", "install", pName, "--grant-all-permissions")
	if err != nil {
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
	assert.Assert(c, strings.Contains(out, volName))
	assert.Assert(c, strings.Contains(out, pName))

	out, err = s.d.Cmd("run", "--rm", "-v", volName+":"+destDir, "busybox", "touch", destDir+destFile)
	assert.NilError(c, err, out)

	out, err = s.d.Cmd("run", "--rm", "-v", volName+":"+destDir, "busybox", "ls", destDir+destFile)
	assert.NilError(c, err, out)
}

func (s *DockerDaemonSuite) TestPluginVolumeRemoveOnRestart(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c, "--live-restore=true")

	out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pName)
	assert.NilError(c, err, out)
	assert.Assert(c, strings.Contains(out, pName))

	out, err = s.d.Cmd("volume", "create", "--driver", pName, "test")
	assert.NilError(c, err, out)

	s.d.Restart(c, "--live-restore=true")

	out, err = s.d.Cmd("plugin", "disable", pName)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "in use"))

	out, err = s.d.Cmd("volume", "rm", "test")
	assert.NilError(c, err, out)

	out, err = s.d.Cmd("plugin", "disable", pName)
	assert.NilError(c, err, out)

	out, err = s.d.Cmd("plugin", "rm", pName)
	assert.NilError(c, err, out)
}

func (s *DockerDaemonSuite) TestPluginListFilterEnabled(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c)

	out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pNameWithTag, "--disable")
	assert.NilError(c, err, out)

	defer func() {
		if out, err := s.d.Cmd("plugin", "remove", pNameWithTag); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	out, err = s.d.Cmd("plugin", "ls", "--filter", "enabled=true")
	assert.NilError(c, err, out)
	assert.Assert(c, !strings.Contains(out, pName))

	out, err = s.d.Cmd("plugin", "ls", "--filter", "enabled=false")
	assert.NilError(c, err, out)
	assert.Assert(c, strings.Contains(out, pName))
	assert.Assert(c, strings.Contains(out, "false"))

	out, err = s.d.Cmd("plugin", "ls")
	assert.NilError(c, err, out)
	assert.Assert(c, strings.Contains(out, pName))
}

func (s *DockerDaemonSuite) TestPluginListFilterCapability(c *testing.T) {
	testRequires(c, IsAmd64, Network)

	s.d.Start(c)

	out, err := s.d.Cmd("plugin", "install", "--grant-all-permissions", pNameWithTag, "--disable")
	assert.NilError(c, err, out)

	defer func() {
		if out, err := s.d.Cmd("plugin", "remove", pNameWithTag); err != nil {
			c.Fatalf("Could not remove plugin: %v %s", err, out)
		}
	}()

	out, err = s.d.Cmd("plugin", "ls", "--filter", "capability=volumedriver")
	assert.NilError(c, err, out)
	assert.Assert(c, strings.Contains(out, pName))

	out, err = s.d.Cmd("plugin", "ls", "--filter", "capability=authz")
	assert.NilError(c, err, out)
	assert.Assert(c, !strings.Contains(out, pName))

	out, err = s.d.Cmd("plugin", "ls")
	assert.NilError(c, err, out)
	assert.Assert(c, strings.Contains(out, pName))
}
