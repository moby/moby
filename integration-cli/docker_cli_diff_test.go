package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

// ensure that an added file shows up in docker diff
func (s *DockerSuite) TestDiffFilenameShownInOutput(c *check.C) {
	containerCmd := `echo foo > /root/bar`
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", containerCmd)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanCID := strings.TrimSpace(out)

	diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
	out, _, err = runCommandWithOutput(diffCmd)
	if err != nil {
		c.Fatalf("failed to run diff: %s %v", out, err)
	}

	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains("A /root/bar", line) {
			found = true
			break
		}
	}
	if !found {
		c.Errorf("couldn't find the new file in docker diff's output: %v", out)
	}
}

// test to ensure GH #3840 doesn't occur any more
func (s *DockerSuite) TestDiffEnsureDockerinitFilesAreIgnored(c *check.C) {
	// this is a list of files which shouldn't show up in `docker diff`
	dockerinitFiles := []string{"/etc/resolv.conf", "/etc/hostname", "/etc/hosts", "/.dockerinit", "/.dockerenv"}
	containerCount := 5

	// we might not run into this problem from the first run, so start a few containers
	for i := 0; i < containerCount; i++ {
		containerCmd := `echo foo > /root/bar`
		runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", containerCmd)
		out, _, err := runCommandWithOutput(runCmd)

		if err != nil {
			c.Fatal(out, err)
		}

		cleanCID := strings.TrimSpace(out)

		diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
		out, _, err = runCommandWithOutput(diffCmd)
		if err != nil {
			c.Fatalf("failed to run diff: %s, %v", out, err)
		}

		for _, filename := range dockerinitFiles {
			if strings.Contains(out, filename) {
				c.Errorf("found file which should've been ignored %v in diff output", filename)
			}
		}
	}
}

func (s *DockerSuite) TestDiffEnsureOnlyKmsgAndPtmx(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sleep", "0")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanCID := strings.TrimSpace(out)

	diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
	out, _, err = runCommandWithOutput(diffCmd)
	if err != nil {
		c.Fatalf("failed to run diff: %s, %v", out, err)
	}

	expected := map[string]bool{
		"C /dev":         true,
		"A /dev/full":    true, // busybox
		"C /dev/ptmx":    true, // libcontainer
		"A /dev/kmsg":    true, // lxc
		"A /dev/fd":      true,
		"A /dev/fuse":    true,
		"A /dev/ptmx":    true,
		"A /dev/null":    true,
		"A /dev/random":  true,
		"A /dev/stdout":  true,
		"A /dev/stderr":  true,
		"A /dev/tty1":    true,
		"A /dev/stdin":   true,
		"A /dev/tty":     true,
		"A /dev/urandom": true,
		"A /dev/zero":    true,
	}

	for _, line := range strings.Split(out, "\n") {
		if line != "" && !expected[line] {
			c.Errorf("%q is shown in the diff but shouldn't", line)
		}
	}
}
