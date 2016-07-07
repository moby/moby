// +build linux, experimental

package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

var pluginName = "tiborvass/no-remove"

// TestDaemonRestartWithPluginEnabled tests state restore for an enabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginEnabled(c *check.C) {
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

// TestDaemonRestartWithPluginEnabled tests state restore for a disabled plugin
func (s *DockerDaemonSuite) TestDaemonRestartWithPluginDisabled(c *check.C) {
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
