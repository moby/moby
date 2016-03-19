// +build daemon,!windows,experimental

package main

import (
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

// TestDaemonRestartWithKilledRunningContainer requires live restore of running containers
func (s *DockerDaemonSuite) TestDaemonRestartWithKilledRunningContainer(t *check.C) {
	// TODO(mlaventure): Not sure what would the exit code be on windows
	testRequires(t, DaemonIsLinux)
	if err := s.d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	cid, err := s.d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	defer s.d.Stop()
	if err != nil {
		t.Fatal(cid, err)
	}
	cid = strings.TrimSpace(cid)

	// Kill the daemon
	if err := s.d.Kill(); err != nil {
		t.Fatal(err)
	}

	// kill the container
	runCmd := exec.Command("ctr", "--address", "/var/run/docker/libcontainerd/containerd.sock", "containers", "kill", cid)
	if out, ec, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatalf("Failed to run ctr, ExitCode: %d, err: '%v' output: '%s' cid: '%s'\n", ec, err, out, cid)
	}

	// Give time to containerd to process the command if we don't
	// the exit event might be received after we do the inspect
	time.Sleep(3 * time.Second)

	// restart the daemon
	if err := s.d.Start(); err != nil {
		t.Fatal(err)
	}

	// Check that we've got the correct exit code
	out, err := s.d.Cmd("inspect", "-f", "{{.State.ExitCode}}", cid)
	t.Assert(err, check.IsNil)

	out = strings.TrimSpace(out)
	if out != "143" {
		t.Fatalf("Expected exit code '%s' got '%s' for container '%s'\n", "143", out, cid)
	}

}

// TestDaemonRestartWithPausedRunningContainer requires live restore of running containers
func (s *DockerDaemonSuite) TestDaemonRestartWithPausedRunningContainer(t *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	cid, err := s.d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	defer s.d.Stop()
	if err != nil {
		t.Fatal(cid, err)
	}
	cid = strings.TrimSpace(cid)

	// Kill the daemon
	if err := s.d.Kill(); err != nil {
		t.Fatal(err)
	}

	// kill the container
	runCmd := exec.Command("ctr", "--address", "/var/run/docker/libcontainerd/containerd.sock", "containers", "pause", cid)
	if out, ec, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatalf("Failed to run ctr, ExitCode: %d, err: '%v' output: '%s' cid: '%s'\n", ec, err, out, cid)
	}

	// Give time to containerd to process the command if we don't
	// the pause event might be received after we do the inspect
	time.Sleep(3 * time.Second)

	// restart the daemon
	if err := s.d.Start(); err != nil {
		t.Fatal(err)
	}

	// Check that we've got the correct status
	out, err := s.d.Cmd("inspect", "-f", "{{.State.Status}}", cid)
	t.Assert(err, check.IsNil)

	out = strings.TrimSpace(out)
	if out != "paused" {
		t.Fatalf("Expected exit code '%s' got '%s' for container '%s'\n", "paused", out, cid)
	}
}

// TestDaemonRestartWithUnpausedRunningContainer requires live restore of running containers.
func (s *DockerDaemonSuite) TestDaemonRestartWithUnpausedRunningContainer(t *check.C) {
	// TODO(mlaventure): Not sure what would the exit code be on windows
	testRequires(t, DaemonIsLinux)
	if err := s.d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	cid, err := s.d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	defer s.d.Stop()
	if err != nil {
		t.Fatal(cid, err)
	}
	cid = strings.TrimSpace(cid)

	// pause the container
	if _, err := s.d.Cmd("pause", cid); err != nil {
		t.Fatal(cid, err)
	}

	// Kill the daemon
	if err := s.d.Kill(); err != nil {
		t.Fatal(err)
	}

	// resume the container
	runCmd := exec.Command("ctr", "--address", "/var/run/docker/libcontainerd/containerd.sock", "containers", "resume", cid)
	if out, ec, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatalf("Failed to run ctr, ExitCode: %d, err: '%v' output: '%s' cid: '%s'\n", ec, err, out, cid)
	}

	// Give time to containerd to process the command if we don't
	// the resume event might be received after we do the inspect
	time.Sleep(3 * time.Second)

	// restart the daemon
	if err := s.d.Start(); err != nil {
		t.Fatal(err)
	}

	// Check that we've got the correct status
	out, err := s.d.Cmd("inspect", "-f", "{{.State.Status}}", cid)
	t.Assert(err, check.IsNil)

	out = strings.TrimSpace(out)
	if out != "running" {
		t.Fatalf("Expected exit code '%s' got '%s' for container '%s'\n", "running", out, cid)
	}
}
