package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestMainHelpWidth(t *testing.T) {
	// Make sure main help text fits within 80 chars and that
	// on non-windows system we use ~ when possible (to shorten things)

	var home string
	if runtime.GOOS != "windows" {
		home = os.Getenv("HOME")
	}

	helpCmd := exec.Command(dockerBinary, "help")
	out, ec, err := runCommandWithOutput(helpCmd)
	if err != nil || ec != 0 {
		t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if len(line) > 80 {
			t.Fatalf("Line is too long(%d chars):\n%s", len(line), line)
		}
		if home != "" && strings.Contains(line, home) {
			t.Fatalf("Line should use ~ instead of %q:\n%s", home, line)
		}
	}
	logDone("help - verify main width")
}

func TestCmdHelpWidth(t *testing.T) {
	// Make sure main help text fits within 80 chars and that
	// on non-windows system we use ~ when possible (to shorten things)

	var home string
	if runtime.GOOS != "windows" {
		home = os.Getenv("HOME")
	}

	for _, command := range []string{
		"attach",
		"build",
		"commit",
		"cp",
		"create",
		"diff",
		"events",
		"exec",
		"export",
		"history",
		"images",
		"import",
		"info",
		"inspect",
		"kill",
		"load",
		"login",
		"logout",
		"logs",
		"port",
		"pause",
		"ps",
		"pull",
		"push",
		"rename",
		"restart",
		"rm",
		"rmi",
		"run",
		"save",
		"search",
		"start",
		"stats",
		"stop",
		"tag",
		"top",
		"unpause",
		"version",
		"wait",
	} {
		helpCmd := exec.Command(dockerBinary, command, "--help")
		out, ec, err := runCommandWithOutput(helpCmd)
		if err != nil || ec != 0 {
			t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
		}
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if len(line) > 80 {
				t.Fatalf("Help for %q is too long(%d chars):\n%s", command, len(line), line)
			}
			if home != "" && strings.Contains(line, home) {
				t.Fatalf("Help for %q should use ~ instead of %q on:\n%s", command, home, line)
			}
		}
	}

	logDone("help - cmd widths")
}
